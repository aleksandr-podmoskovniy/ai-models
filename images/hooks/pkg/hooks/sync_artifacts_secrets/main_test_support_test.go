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

package sync_artifacts_secrets

import (
	"strings"
	"testing"

	"github.com/deckhouse/module-sdk/pkg"
	patchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"
	sdktest "github.com/deckhouse/module-sdk/testing/mock"
)

func newValues(t *testing.T, raw map[string]any) *patchablevalues.PatchableValues {
	t.Helper()

	values, err := patchablevalues.NewPatchableValues(raw)
	if err != nil {
		t.Fatalf("new patchable values: %v", err)
	}

	return values
}

func newSecretSnapshot(t *testing.T, secret sourceSecretSnapshot) pkg.Snapshot {
	t.Helper()

	snapshot := sdktest.NewSnapshotMock(t)
	snapshot.UnmarshalToMock.Set(func(v any) error {
		target, ok := v.(*sourceSecretSnapshot)
		if !ok {
			t.Fatalf("unexpected snapshot target type %T", v)
		}
		*target = secret
		return nil
	})
	snapshot.StringMock.Optional().Return("")
	return snapshot
}

func newNamespaceSnapshot(t *testing.T) pkg.Snapshot {
	t.Helper()

	snapshot := sdktest.NewSnapshotMock(t)
	snapshot.StringMock.Optional().Return("")
	return snapshot
}

func newSnapshotsMock(t *testing.T, moduleNamespacePresent bool, secrets ...sourceSecretSnapshot) *sdktest.SnapshotsMock {
	t.Helper()

	snapshots := sdktest.NewSnapshotsMock(t)
	snapshots.GetMock.Set(func(key string) []pkg.Snapshot {
		switch key {
		case moduleNamespaceSnapshotName:
			if !moduleNamespacePresent {
				return nil
			}
			return []pkg.Snapshot{newNamespaceSnapshot(t)}
		case secretsSnapshotName:
			result := make([]pkg.Snapshot, 0, len(secrets))
			for _, secret := range secrets {
				result = append(result, newSecretSnapshot(t, secret))
			}
			return result
		default:
			t.Fatalf("unexpected snapshot key %q", key)
			return nil
		}
	})

	return snapshots
}

func assertValuePatchExists(t *testing.T, values *patchablevalues.PatchableValues, path, expectedValue string) {
	t.Helper()

	for _, patch := range values.GetPatches() {
		if patch.Path != "/"+pathToJSONPointer(path) {
			continue
		}
		if patch.Op != "add" {
			t.Fatalf("unexpected patch operation for %s: %s", path, patch.Op)
		}
		if string(patch.Value) != expectedValue {
			t.Fatalf("unexpected patch value for %s: %s", path, patch.Value)
		}
		return
	}

	t.Fatalf("expected patch for %s", path)
}

func assertValueRemovePatchExists(t *testing.T, values *patchablevalues.PatchableValues, path string) {
	t.Helper()

	for _, patch := range values.GetPatches() {
		if patch.Path != "/"+pathToJSONPointer(path) {
			continue
		}
		if patch.Op != "remove" {
			t.Fatalf("unexpected patch operation for %s: %s", path, patch.Op)
		}
		return
	}

	t.Fatalf("expected remove patch for %s", path)
}

func pathToJSONPointer(path string) string {
	return strings.ReplaceAll(path, ".", "/")
}
