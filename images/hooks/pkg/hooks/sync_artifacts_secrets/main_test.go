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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/module-sdk/pkg"
	sdktest "github.com/deckhouse/module-sdk/testing/mock"
)

func TestReconcileCopiesCredentialsSecretAndUsesEmbeddedCA(t *testing.T) {
	t.Parallel()

	values := newValues(t, map[string]any{
		"aiModels": map[string]any{
			"artifacts": map[string]any{
				"credentialsSecretName": "s3-credentials",
				"caSecretName":          "",
			},
			"internal": map[string]any{
				"artifacts": map[string]any{},
			},
		},
	})

	snapshots := newSnapshotsMock(t, true,
		sourceSecretSnapshot{
			Name: "s3-credentials",
			Data: map[string][]byte{
				"accessKey": []byte("AKIA"),
				"secretKey": []byte("SECRET"),
				"ca.crt":    []byte("CA"),
			},
		},
	)

	var created []*corev1.Secret
	patchCollector := sdktest.NewPatchCollectorMock(t)
	patchCollector.CreateOrUpdateMock.Set(func(object any) {
		secret, ok := object.(*corev1.Secret)
		if !ok {
			t.Fatalf("unexpected object type %T", object)
		}
		created = append(created, secret)
	})
	patchCollector.DeleteInBackgroundMock.Optional().Set(func(apiVersion, kind, namespace, name string) {})

	input := &pkg.HookInput{
		Snapshots:      snapshots,
		Values:         values,
		PatchCollector: patchCollector,
	}

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(created) != 1 {
		t.Fatalf("expected 1 created secret, got %d", len(created))
	}

	secret := created[0]
	if secret.Name != syncedCredentialsSecretName {
		t.Fatalf("unexpected secret name %q", secret.Name)
	}
	if got := string(secret.Data["accessKey"]); got != "AKIA" {
		t.Fatalf("unexpected accessKey %q", got)
	}
	if got := string(secret.Data["secretKey"]); got != "SECRET" {
		t.Fatalf("unexpected secretKey %q", got)
	}
	if got := string(secret.Data["ca.crt"]); got != "CA" {
		t.Fatalf("unexpected ca.crt %q", got)
	}

	assertValuePatchExists(t, values, internalSyncedCredentialsSecretNamePath, `"`+syncedCredentialsSecretName+`"`)
	assertValuePatchExists(t, values, internalMountedCASecretNamePath, `"`+syncedCredentialsSecretName+`"`)
}

func TestReconcileCopiesSeparateCASecret(t *testing.T) {
	t.Parallel()

	values := newValues(t, map[string]any{
		"aiModels": map[string]any{
			"artifacts": map[string]any{
				"credentialsSecretName": "s3-credentials",
				"caSecretName":          "s3-ca",
			},
			"internal": map[string]any{
				"artifacts": map[string]any{},
			},
		},
	})

	snapshots := newSnapshotsMock(t, true,
		sourceSecretSnapshot{
			Name: "s3-credentials",
			Data: map[string][]byte{
				"accessKey": []byte("AKIA"),
				"secretKey": []byte("SECRET"),
			},
		},
		sourceSecretSnapshot{
			Name: "s3-ca",
			Data: map[string][]byte{
				"ca.crt": []byte("CUSTOM-CA"),
			},
		},
	)

	var created []*corev1.Secret
	patchCollector := sdktest.NewPatchCollectorMock(t)
	patchCollector.CreateOrUpdateMock.Set(func(object any) {
		secret, ok := object.(*corev1.Secret)
		if !ok {
			t.Fatalf("unexpected object type %T", object)
		}
		created = append(created, secret)
	})
	patchCollector.DeleteInBackgroundMock.Optional().Set(func(apiVersion, kind, namespace, name string) {})

	input := &pkg.HookInput{
		Snapshots:      snapshots,
		Values:         values,
		PatchCollector: patchCollector,
	}

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(created) != 2 {
		t.Fatalf("expected 2 created secrets, got %d", len(created))
	}

	if created[0].Name != syncedCredentialsSecretName {
		t.Fatalf("unexpected first secret name %q", created[0].Name)
	}
	if created[1].Name != syncedCASecretName {
		t.Fatalf("unexpected second secret name %q", created[1].Name)
	}
	if got := string(created[1].Data["ca.crt"]); got != "CUSTOM-CA" {
		t.Fatalf("unexpected copied ca.crt %q", got)
	}

	assertValuePatchExists(t, values, internalMountedCASecretNamePath, `"`+syncedCASecretName+`"`)
}

func TestReconcileFailsWhenCredentialsSecretIsMissing(t *testing.T) {
	t.Parallel()

	values := newValues(t, map[string]any{
		"aiModels": map[string]any{
			"artifacts": map[string]any{
				"credentialsSecretName": "missing",
			},
			"internal": map[string]any{
				"artifacts": map[string]any{},
			},
		},
	})

	snapshots := newSnapshotsMock(t, true)

	patchCollector := sdktest.NewPatchCollectorMock(t)
	patchCollector.CreateOrUpdateMock.Optional()
	patchCollector.DeleteInBackgroundMock.Optional().Set(func(apiVersion, kind, namespace, name string) {})

	input := &pkg.HookInput{
		Snapshots:      snapshots,
		Values:         values,
		PatchCollector: patchCollector,
	}

	err := Reconcile(context.Background(), input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "artifacts credentials secret d8-system/missing not found" {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestReconcileDefersSecretSyncUntilModuleNamespaceExists(t *testing.T) {
	t.Parallel()

	values := newValues(t, map[string]any{
		"aiModels": map[string]any{
			"artifacts": map[string]any{
				"credentialsSecretName": "s3-credentials",
				"caSecretName":          "s3-ca",
			},
			"internal": map[string]any{
				"artifacts": map[string]any{},
			},
		},
	})

	snapshots := newSnapshotsMock(t, false,
		sourceSecretSnapshot{
			Name: "s3-credentials",
			Data: map[string][]byte{
				"accessKey": []byte("AKIA"),
				"secretKey": []byte("SECRET"),
			},
		},
		sourceSecretSnapshot{
			Name: "s3-ca",
			Data: map[string][]byte{
				"ca.crt": []byte("CUSTOM-CA"),
			},
		},
	)

	patchCollector := sdktest.NewPatchCollectorMock(t)
	patchCollector.CreateOrUpdateMock.Optional().Set(func(object any) {
		t.Fatalf("unexpected CreateOrUpdate for %T", object)
	})
	patchCollector.DeleteInBackgroundMock.Optional().Set(func(apiVersion, kind, namespace, name string) {
		t.Fatalf("unexpected DeleteInBackground for %s/%s", namespace, name)
	})

	input := &pkg.HookInput{
		Snapshots:      snapshots,
		Values:         values,
		PatchCollector: patchCollector,
	}

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertValuePatchExists(t, values, internalSyncedCredentialsSecretNamePath, `"`+syncedCredentialsSecretName+`"`)
	assertValuePatchExists(t, values, internalMountedCASecretNamePath, `"`+syncedCASecretName+`"`)
}

func TestReconcileSkipsDeleteUntilModuleNamespaceExists(t *testing.T) {
	t.Parallel()

	values := newValues(t, map[string]any{
		"aiModels": map[string]any{
			"artifacts": map[string]any{
				"credentialsSecretName": "",
			},
			"internal": map[string]any{
				"artifacts": map[string]any{
					"mountedCASecretName": syncedCASecretName,
				},
			},
		},
	})

	snapshots := newSnapshotsMock(t, false)

	patchCollector := sdktest.NewPatchCollectorMock(t)
	patchCollector.CreateOrUpdateMock.Optional().Set(func(object any) {
		t.Fatalf("unexpected CreateOrUpdate for %T", object)
	})
	patchCollector.DeleteInBackgroundMock.Optional().Set(func(apiVersion, kind, namespace, name string) {
		t.Fatalf("unexpected DeleteInBackground for %s/%s", namespace, name)
	})

	input := &pkg.HookInput{
		Snapshots:      snapshots,
		Values:         values,
		PatchCollector: patchCollector,
	}

	if err := Reconcile(context.Background(), input); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertValuePatchExists(t, values, internalSyncedCredentialsSecretNamePath, `"`+syncedCredentialsSecretName+`"`)
	assertValueRemovePatchExists(t, values, internalMountedCASecretNamePath)
}
