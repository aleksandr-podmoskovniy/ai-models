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

package sourceworker

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceGetOrCreateSetsControllerOwnerReference(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	service, err := NewService(kubeClient, scheme, Options{
		Namespace:             "d8-ai-models",
		Image:                 "backend:latest",
		ServiceAccountName:    "ai-models-controller",
		OCIRepositoryPrefix:   "registry.internal.local/ai-models",
		OCIRegistrySecretName: "ai-models-publication-registry",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	operation := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ai-model-publication-1111-2222",
			Namespace: "d8-ai-models",
			UID:       types.UID("operation-uid"),
		},
	}
	request := testOperationContext()
	request.OperationName = ""
	request.OperationNamespace = ""

	handle, created, err := service.GetOrCreate(context.Background(), operation, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected pod to be created")
	}
	if handle == nil {
		t.Fatal("expected source worker handle")
	}

	pod := &corev1.Pod{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: handle.Name, Namespace: "d8-ai-models"}, pod); err != nil {
		t.Fatalf("Get(stored pod) error = %v", err)
	}
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("unexpected owner reference count %d", len(pod.OwnerReferences))
	}
	owner := pod.OwnerReferences[0]
	if owner.Kind != "ConfigMap" || owner.Name != operation.Name || owner.UID != operation.UID {
		t.Fatalf("unexpected owner reference %#v", owner)
	}

	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("unexpected stored owner reference count %d", len(pod.OwnerReferences))
	}
}

func TestServiceGetOrCreateProjectsSourceAuthSecret(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}

	operation := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ai-model-publication-2222-3333",
			Namespace: "d8-ai-models",
			UID:       types.UID("operation-auth-uid"),
		},
	}
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hf-auth",
			Namespace: "team-a",
		},
		Data: map[string][]byte{
			"token":        []byte("hf-token"),
			"unused-extra": []byte("must-not-be-copied"),
		},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(operation, sourceSecret).
		Build()

	service, err := NewService(kubeClient, scheme, Options{
		Namespace:             "d8-ai-models",
		Image:                 "backend:latest",
		ServiceAccountName:    "ai-models-controller",
		OCIRepositoryPrefix:   "registry.internal.local/ai-models",
		OCIRegistrySecretName: "ai-models-publication-registry",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	request := testOperationContext()
	request.Request.Owner.UID = types.UID("2222-3333")
	request.Request.Owner.Name = "deepseek-r1-private"
	request.Request.Identity.Name = "deepseek-r1-private"
	request.Request.Spec.Source.HuggingFace.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "hf-auth"}
	request.OperationName = ""
	request.OperationNamespace = ""

	handle, created, err := service.GetOrCreate(context.Background(), operation, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected pod to be created")
	}
	if handle == nil {
		t.Fatal("expected source worker handle")
	}

	pod := &corev1.Pod{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: handle.Name, Namespace: "d8-ai-models"}, pod); err != nil {
		t.Fatalf("Get(stored pod) error = %v", err)
	}

	secretName, err := resourcenames.SourceWorkerAuthSecretName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerAuthSecretName() error = %v", err)
	}

	projected := &corev1.Secret{}
	if err := kubeClient.Get(
		context.Background(),
		client.ObjectKey{Name: secretName, Namespace: "d8-ai-models"},
		projected,
	); err != nil {
		t.Fatalf("Get(projected secret) error = %v", err)
	}
	if got, want := string(projected.Data["token"]), "hf-token"; got != want {
		t.Fatalf("unexpected projected token %q", got)
	}
	if _, found := projected.Data["unused-extra"]; found {
		t.Fatal("projected secret must not copy unsupported keys")
	}
	if len(projected.OwnerReferences) != 1 {
		t.Fatalf("unexpected projected secret owner reference count %d", len(projected.OwnerReferences))
	}

	if err := handle.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := kubeClient.Get(
		context.Background(),
		client.ObjectKey{Name: secretName, Namespace: "d8-ai-models"},
		projected,
	); !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected secret to be deleted, got err=%v", err)
	}
}
