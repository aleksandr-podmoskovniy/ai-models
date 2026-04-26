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

package root_ca_discovery

import (
	"context"
	"testing"

	"github.com/deckhouse/module-sdk/pkg"
	patchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"
	sdktest "github.com/deckhouse/module-sdk/testing/mock"
)

func TestReconcileStoresExistingRootCA(t *testing.T) {
	t.Parallel()

	values := newValues(t)
	input := &pkg.HookInput{
		Snapshots: newSnapshots(t, caSecret{
			Crt: []byte("ROOT-CA"),
			Key: []byte("ROOT-KEY"),
		}),
		Values: values,
	}

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	for _, patch := range values.GetPatches() {
		if patch.Path == "/aiModels/internal/rootCA" && patch.Op == "add" {
			return
		}
	}
	t.Fatal("expected rootCA values patch")
}

func TestReconcileNoopsWhenSecretMissing(t *testing.T) {
	t.Parallel()

	values := newValues(t)
	input := &pkg.HookInput{
		Snapshots: newSnapshots(t),
		Values:    values,
	}

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(values.GetPatches()) != 0 {
		t.Fatalf("unexpected patches: %#v", values.GetPatches())
	}
}

func newValues(t *testing.T) *patchablevalues.PatchableValues {
	t.Helper()

	values, err := patchablevalues.NewPatchableValues(map[string]any{})
	if err != nil {
		t.Fatalf("new patchable values: %v", err)
	}
	return values
}

func newSnapshots(t *testing.T, secrets ...caSecret) *sdktest.SnapshotsMock {
	t.Helper()

	snapshots := sdktest.NewSnapshotsMock(t)
	snapshots.GetMock.Set(func(key string) []pkg.Snapshot {
		if key != rootCASnapshotName {
			t.Fatalf("unexpected snapshot key %q", key)
		}
		result := make([]pkg.Snapshot, 0, len(secrets))
		for _, secret := range secrets {
			result = append(result, newSnapshot(t, secret))
		}
		return result
	})
	return snapshots
}

func newSnapshot(t *testing.T, secret caSecret) pkg.Snapshot {
	t.Helper()

	snapshot := sdktest.NewSnapshotMock(t)
	snapshot.UnmarshalToMock.Set(func(v any) error {
		target, ok := v.(*caSecret)
		if !ok {
			t.Fatalf("unexpected snapshot target type %T", v)
		}
		*target = secret
		return nil
	})
	snapshot.StringMock.Optional().Return("")
	return snapshot
}
