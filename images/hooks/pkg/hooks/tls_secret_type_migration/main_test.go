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

package tls_secret_type_migration

import (
	"context"
	"testing"

	"github.com/deckhouse/module-sdk/pkg"
	sdktest "github.com/deckhouse/module-sdk/testing/mock"

	"hooks/pkg/settings"
)

func TestReconcileDeletesLegacyOpaqueTLSSecrets(t *testing.T) {
	t.Parallel()

	var deleted []string
	input := newInput(t,
		func(apiVersion, kind, namespace, name string) {
			if apiVersion != secretAPIVersion {
				t.Fatalf("unexpected apiVersion %q", apiVersion)
			}
			if kind != secretKind {
				t.Fatalf("unexpected kind %q", kind)
			}
			if namespace != settings.ModuleNamespace {
				t.Fatalf("unexpected namespace %q", namespace)
			}
			deleted = append(deleted, name)
		},
		tlsSecretSnapshot{Name: webhookTLSSecretName, Type: "Opaque"},
		tlsSecretSnapshot{Name: dmcrTLSSecretName, Type: "Opaque"},
	)

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(deleted) != 2 {
		t.Fatalf("expected 2 deleted secrets, got %d", len(deleted))
	}
	if deleted[0] != webhookTLSSecretName {
		t.Fatalf("unexpected first deleted secret %q", deleted[0])
	}
	if deleted[1] != dmcrTLSSecretName {
		t.Fatalf("unexpected second deleted secret %q", deleted[1])
	}
}

func TestHookRunsAfterCommonTLSHooks(t *testing.T) {
	t.Parallel()

	if config.OnBeforeHelm == nil {
		t.Fatal("expected OnBeforeHelm config")
	}
	if config.OnBeforeHelm.Order != tlsSecretMigrationOrder {
		t.Fatalf("unexpected hook order %d", config.OnBeforeHelm.Order)
	}
	if tlsSecretMigrationOrder <= 5 {
		t.Fatalf("migration order %d must be after common TLS hook order 5", tlsSecretMigrationOrder)
	}
}

func TestReconcilePreservesKubernetesTLSSecrets(t *testing.T) {
	t.Parallel()

	patchCollector := sdktest.NewPatchCollectorMock(t)
	patchCollector.DeleteInBackgroundMock.Optional().Set(func(apiVersion, kind, namespace, name string) {})

	input := &pkg.HookInput{
		Snapshots: newSnapshots(t,
			tlsSecretSnapshot{Name: webhookTLSSecretName, Type: expectedKubernetesTLSRaw},
			tlsSecretSnapshot{Name: dmcrTLSSecretName, Type: expectedKubernetesTLSRaw},
		),
		PatchCollector: patchCollector,
	}

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if got := patchCollector.DeleteInBackgroundAfterCounter(); got != 0 {
		t.Fatalf("expected no deletes, got %d", got)
	}
}

func TestReconcileIgnoresUnknownSnapshotNames(t *testing.T) {
	t.Parallel()

	patchCollector := sdktest.NewPatchCollectorMock(t)
	patchCollector.DeleteInBackgroundMock.Optional().Set(func(apiVersion, kind, namespace, name string) {})

	input := &pkg.HookInput{
		Snapshots:      newSnapshots(t, tlsSecretSnapshot{Name: "other", Type: "Opaque"}),
		PatchCollector: patchCollector,
	}

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if got := patchCollector.DeleteInBackgroundAfterCounter(); got != 0 {
		t.Fatalf("expected no deletes, got %d", got)
	}
}

func newInput(t *testing.T, onDelete func(apiVersion, kind, namespace, name string), secrets ...tlsSecretSnapshot) *pkg.HookInput {
	t.Helper()

	patchCollector := sdktest.NewPatchCollectorMock(t)
	patchCollector.DeleteInBackgroundMock.Set(onDelete)

	return &pkg.HookInput{
		Snapshots:      newSnapshots(t, secrets...),
		PatchCollector: patchCollector,
	}
}

func newSnapshots(t *testing.T, secrets ...tlsSecretSnapshot) *sdktest.SnapshotsMock {
	t.Helper()

	snapshots := sdktest.NewSnapshotsMock(t)
	snapshots.GetMock.Set(func(key string) []pkg.Snapshot {
		if key != tlsSecretsSnapshotName {
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

func newSnapshot(t *testing.T, secret tlsSecretSnapshot) pkg.Snapshot {
	t.Helper()

	snapshot := sdktest.NewSnapshotMock(t)
	snapshot.UnmarshalToMock.Set(func(v any) error {
		target, ok := v.(*tlsSecretSnapshot)
		if !ok {
			t.Fatalf("unexpected snapshot target type %T", v)
		}
		*target = secret
		return nil
	})
	snapshot.StringMock.Optional().Return("")
	return snapshot
}
