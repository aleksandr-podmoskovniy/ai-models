/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generate_dmcr_auth

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/deckhouse/module-sdk/pkg"
	patchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"
	sdktest "github.com/deckhouse/module-sdk/testing/mock"
)

func TestReconcilePreservesExistingAuthSecret(t *testing.T) {
	t.Parallel()

	writePassword := "A1aExistingWritePassword000000000000000001"
	readPassword := "A1aExistingReadPassword000000000000000002"
	writeHtpasswd, err := generateHtpasswd(writeUsername, writePassword)
	if err != nil {
		t.Fatal(err)
	}
	readHtpasswd, err := generateHtpasswd(readUsername, readPassword)
	if err != nil {
		t.Fatal(err)
	}

	values := newValues(t, nil)
	input := newInput(t, values, newSnapshot(t, authSecretName, map[string]string{
		"write.password": writePassword,
		"read.password":  readPassword,
		"write.htpasswd": writeHtpasswd,
		"read.htpasswd":  readHtpasswd,
		"salt":           "existing-salt",
	}))

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertValue(t, values, writePasswordValuesPath, writePassword)
	assertValue(t, values, readPasswordValuesPath, readPassword)
	assertValue(t, values, writeHtpasswdValuesPath, writeHtpasswd)
	assertValue(t, values, readHtpasswdValuesPath, readHtpasswd)
	assertValue(t, values, saltValuesPath, "existing-salt")
}

func TestReconcileFallsBackToDockerConfigSecrets(t *testing.T) {
	t.Parallel()

	writePassword := "A1aExistingWriteOnly00000000000000000001"
	readPassword := "A1aExistingReadOnly000000000000000000002"

	values := newValues(t, nil)
	input := newInput(t, values,
		newSnapshot(t, writeAuthSecretName, map[string]string{"password": writePassword}),
		newSnapshot(t, readAuthSecretName, map[string]string{"password": readPassword}),
	)

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	state := stateFromPatches(t, values)
	if state.WritePassword != writePassword {
		t.Fatalf("unexpected write password %q", state.WritePassword)
	}
	if state.ReadPassword != readPassword {
		t.Fatalf("unexpected read password %q", state.ReadPassword)
	}
	if !validateHtpasswd(writeUsername, writePassword, state.WriteHtpasswd) {
		t.Fatal("write htpasswd does not match migrated password")
	}
	if !validateHtpasswd(readUsername, readPassword, state.ReadHtpasswd) {
		t.Fatal("read htpasswd does not match migrated password")
	}
	if len(state.Salt) != saltLength {
		t.Fatalf("unexpected salt length %d", len(state.Salt))
	}
}

func TestReconcileRegeneratesInvalidState(t *testing.T) {
	t.Parallel()

	values := newValues(t, map[string]any{
		"aiModels": map[string]any{
			"internal": map[string]any{
				"dmcr": map[string]any{
					"auth": map[string]any{
						"writePassword": "A1aExistingWritePassword000000000000000001",
						"readPassword":  "A1aExistingReadPassword000000000000000002",
						"writeHtpasswd": "not-valid",
						"readHtpasswd":  "not-valid",
						"salt":          "",
					},
				},
			},
		},
	})
	input := newInput(t, values)

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	state := stateFromPatches(t, values)
	if !validateHtpasswd(writeUsername, state.WritePassword, state.WriteHtpasswd) {
		t.Fatal("write htpasswd was not regenerated")
	}
	if !validateHtpasswd(readUsername, state.ReadPassword, state.ReadHtpasswd) {
		t.Fatal("read htpasswd was not regenerated")
	}
	if len(state.Salt) != saltLength {
		t.Fatalf("unexpected salt length %d", len(state.Salt))
	}
}

func newInput(t *testing.T, values *patchablevalues.PatchableValues, snapshots ...pkg.Snapshot) *pkg.HookInput {
	t.Helper()

	snapshotMock := sdktest.NewSnapshotsMock(t)
	snapshotMock.GetMock.Set(func(key string) []pkg.Snapshot {
		if key != authSecretSnapshotName {
			t.Fatalf("unexpected snapshot key %q", key)
		}
		return snapshots
	})

	return &pkg.HookInput{
		Snapshots: snapshotMock,
		Values:    values,
	}
}

func newValues(t *testing.T, raw map[string]any) *patchablevalues.PatchableValues {
	t.Helper()

	if raw == nil {
		raw = map[string]any{}
	}
	values, err := patchablevalues.NewPatchableValues(raw)
	if err != nil {
		t.Fatalf("new patchable values: %v", err)
	}
	return values
}

func newSnapshot(t *testing.T, name string, data map[string]string) pkg.Snapshot {
	t.Helper()

	encoded := make(map[string]string, len(data))
	for key, value := range data {
		encoded[key] = base64.StdEncoding.EncodeToString([]byte(value))
	}

	snapshot := sdktest.NewSnapshotMock(t)
	snapshot.UnmarshalToMock.Set(func(v any) error {
		target, ok := v.(*secretSnapshot)
		if !ok {
			t.Fatalf("unexpected snapshot target type %T", v)
		}
		target.Name = name
		target.Data = encoded
		return nil
	})
	snapshot.StringMock.Optional().Return("")
	return snapshot
}

func assertValue(t *testing.T, values *patchablevalues.PatchableValues, path, expected string) {
	t.Helper()

	for _, patch := range values.GetPatches() {
		if patch.Path != "/"+strings.ReplaceAll(path, ".", "/") {
			continue
		}
		if got := strings.Trim(string(patch.Value), `"`); got != expected {
			t.Fatalf("unexpected value for %s: %q", path, got)
		}
		return
	}
	t.Fatalf("expected patch for %s", path)
}

func stateFromPatches(t *testing.T, values *patchablevalues.PatchableValues) authState {
	t.Helper()

	var state authState
	for _, patch := range values.GetPatches() {
		value := strings.Trim(string(patch.Value), `"`)
		switch patch.Path {
		case "/" + strings.ReplaceAll(writePasswordValuesPath, ".", "/"):
			state.WritePassword = value
		case "/" + strings.ReplaceAll(readPasswordValuesPath, ".", "/"):
			state.ReadPassword = value
		case "/" + strings.ReplaceAll(writeHtpasswdValuesPath, ".", "/"):
			state.WriteHtpasswd = value
		case "/" + strings.ReplaceAll(readHtpasswdValuesPath, ".", "/"):
			state.ReadHtpasswd = value
		case "/" + strings.ReplaceAll(saltValuesPath, ".", "/"):
			state.Salt = value
		}
	}
	return state
}
