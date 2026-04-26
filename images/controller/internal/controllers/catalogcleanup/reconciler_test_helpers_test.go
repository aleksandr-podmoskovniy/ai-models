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

package catalogcleanup

import (
	"context"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/cleanupstate"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type recordingCleaner struct {
	calls []cleanuphandle.Handle
	err   error
}

func (c *recordingCleaner) Cleanup(_ context.Context, handle cleanuphandle.Handle) error {
	c.calls = append(c.calls, handle)
	return c.err
}

func newModelReconciler(t *testing.T, objects ...client.Object) (*ModelReconciler, client.Client) {
	t.Helper()
	return newModelReconcilerWithCleaner(t, &recordingCleaner{}, objects...)
}

func newModelReconcilerWithCleaner(t *testing.T, cleaner ArtifactCleaner, objects ...client.Object) (*ModelReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t, corev1.AddToScheme)
	initialObjects := append([]client.Object{
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
	}, cleanupStateSecretsFromObjects(t, objects)...)
	initialObjects = append(initialObjects, objects...)

	kubeClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		initialObjects...,
	)
	cleanupState, err := cleanupstate.New(kubeClient, "d8-ai-models")
	if err != nil {
		t.Fatalf("cleanupstate.New() error = %v", err)
	}

	return &ModelReconciler{baseReconciler{
		client:       kubeClient,
		scheme:       scheme,
		options:      testCleanupOptions(cleaner),
		cleanupState: cleanupState,
	}}, kubeClient
}

func newClusterModelReconciler(t *testing.T, objects ...client.Object) (*ClusterModelReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t, corev1.AddToScheme)
	initialObjects := append([]client.Object{
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
	}, cleanupStateSecretsFromObjects(t, objects)...)
	initialObjects = append(initialObjects, objects...)

	kubeClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		initialObjects...,
	)
	cleanupState, err := cleanupstate.New(kubeClient, "d8-ai-models")
	if err != nil {
		t.Fatalf("cleanupstate.New() error = %v", err)
	}

	return &ClusterModelReconciler{baseReconciler{
		client:       kubeClient,
		scheme:       scheme,
		options:      testCleanupOptions(&recordingCleaner{}),
		cleanupState: cleanupState,
	}}, kubeClient
}

func testCleanupOptions(cleaner ArtifactCleaner) Options {
	return Options{
		Cleanup: CleanupOptions{
			Namespace: "d8-ai-models",
			Cleaner:   cleaner,
		},
		RuntimeNamespace: "d8-ai-models",
		RequeueAfter:     time.Second,
	}
}

func testModel() *modelsv1alpha1.Model {
	return testkit.NewModel()
}

func testClusterModel() *modelsv1alpha1.ClusterModel {
	return testkit.NewClusterModel()
}

func newDeletingModel() *modelsv1alpha1.Model {
	object := testModel()
	now := metav1.Now()
	object.DeletionTimestamp = &now
	object.Finalizers = []string{Finalizer}
	return object
}

func setCleanupHandle(t *testing.T, object metav1.Object, reference string) {
	t.Helper()

	if err := cleanuphandle.SetOnObject(object, cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Artifact: &cleanuphandle.ArtifactSnapshot{
			Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:  reference,
		},
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: reference,
		},
	}); err != nil {
		t.Fatalf("SetOnObject() error = %v", err)
	}
}

func cleanupStateSecretsFromObjects(t *testing.T, objects []client.Object) []client.Object {
	t.Helper()

	secrets := make([]client.Object, 0, len(objects))
	for _, object := range objects {
		if object == nil {
			continue
		}
		raw := object.GetAnnotations()[cleanuphandle.AnnotationKey]
		if raw == "" {
			continue
		}
		owner, err := cleanupOwnerFor(object)
		if err != nil {
			t.Fatalf("cleanupOwnerFor() error = %v", err)
		}
		name, err := resourcenames.CleanupHandleSecretName(object.GetUID())
		if err != nil {
			t.Fatalf("CleanupHandleSecretName() error = %v", err)
		}
		secrets = append(secrets, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   "d8-ai-models",
				Labels:      resourcenames.OwnerLabels(cleanupstate.AppName, owner.Kind, owner.Name, owner.UID, owner.Namespace),
				Annotations: resourcenames.OwnerAnnotations(owner.Kind, owner.Name, owner.Namespace),
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{cleanupstate.DataKey: []byte(raw)},
		})
	}
	return secrets
}
