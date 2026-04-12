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
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceGetOrCreateEncodesOwnerIdentityOnPod(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write")).
		Build()

	service, err := NewService(kubeClient, scheme, testOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	modelOwner := testkit.NewModel()
	request := testOperationRequest()

	handle, created, err := service.GetOrCreate(context.Background(), modelOwner, request)
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
	if len(pod.OwnerReferences) != 0 {
		t.Fatalf("expected no cross-namespace owner reference, got %d", len(pod.OwnerReferences))
	}
	if got, want := pod.Annotations[resourcenames.OwnerNameAnnotationKey], modelOwner.Name; got != want {
		t.Fatalf("unexpected owner-name annotation %q", got)
	}
	if got, want := pod.Annotations[resourcenames.OwnerNamespaceAnnotationKey], modelOwner.Namespace; got != want {
		t.Fatalf("unexpected owner-namespace annotation %q", got)
	}
}

func TestServiceGetOrCreateProjectsSourceAuthSecret(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	modelOwner := testkit.NewModel()
	modelOwner.UID = types.UID("2222-3333")
	modelOwner.Name = "deepseek-r1-private"

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
		WithObjects(modelOwner, sourceSecret, testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write")).
		Build()

	service, err := NewService(kubeClient, scheme, testOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	request := testOperationRequest()
	request.Owner.UID = types.UID("2222-3333")
	request.Owner.Name = "deepseek-r1-private"
	request.Identity.Name = "deepseek-r1-private"
	request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "hf-auth"}

	handle, created, err := service.GetOrCreate(context.Background(), modelOwner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected pod to be created")
	}
	if handle == nil {
		t.Fatal("expected source worker handle")
	}

	secretName, err := resourcenames.SourceWorkerAuthSecretName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerAuthSecretName() error = %v", err)
	}

	projected := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: secretName, Namespace: "d8-ai-models"}, projected); err != nil {
		t.Fatalf("Get(projected secret) error = %v", err)
	}
	if got, want := string(projected.Data["token"]), "hf-token"; got != want {
		t.Fatalf("unexpected projected token %q", got)
	}
	if _, found := projected.Data["unused-extra"]; found {
		t.Fatal("projected secret must not copy unsupported keys")
	}
	if len(projected.OwnerReferences) != 0 {
		t.Fatalf("expected no cross-namespace owner reference on projected secret, got %d", len(projected.OwnerReferences))
	}
	registrySecretName, err := resourcenames.OCIRegistryAuthSecretName(request.Owner.UID)
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	registrySecret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: registrySecretName, Namespace: "d8-ai-models"}, registrySecret); err != nil {
		t.Fatalf("Get(projected OCI auth secret) error = %v", err)
	}
	if got, want := string(registrySecret.Data["username"]), "ai-models"; got != want {
		t.Fatalf("unexpected projected OCI username %q", got)
	}
	if got, want := string(registrySecret.Data["password"]), "secret"; got != want {
		t.Fatalf("unexpected projected OCI password %q", got)
	}

	if err := handle.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: secretName, Namespace: "d8-ai-models"}, projected); !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected secret to be deleted, got err=%v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: registrySecretName, Namespace: "d8-ai-models"}, registrySecret); !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected OCI auth secret to be deleted, got err=%v", err)
	}
}

func TestServiceGetOrCreateQueuesWhenPublishConcurrencyLimitIsReached(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	request := testOperationRequest()
	owner := testkit.NewModel()
	owner.UID = request.Owner.UID
	owner.Name = request.Owner.Name

	busyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ai-model-publish-busy",
			Namespace: "d8-ai-models",
			Labels: resourcenames.OwnerLabels(
				"ai-models-publication",
				modelsv1alpha1.ModelKind,
				"busy-model",
				types.UID("busy-owner"),
				"team-a",
			),
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner, busyPod, testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write")).
		Build()

	options := testOptions()
	options.MaxConcurrentWorkers = 1

	service, err := NewService(kubeClient, scheme, options)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if created {
		t.Fatal("did not expect pod creation while concurrency limit is reached")
	}
	if handle == nil {
		t.Fatal("expected queued handle")
	}
	if got, want := handle.Phase, corev1.PodPending; got != want {
		t.Fatalf("unexpected queued phase %q", got)
	}
	wantName, err := resourcenames.SourceWorkerPodName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerPodName() error = %v", err)
	}
	if got := handle.Name; got != wantName {
		t.Fatalf("unexpected queued worker name %q", got)
	}

	var pods corev1.PodList
	if err := kubeClient.List(context.Background(), &pods, client.InNamespace("d8-ai-models")); err != nil {
		t.Fatalf("List(pods) error = %v", err)
	}
	if got, want := len(pods.Items), 1; got != want {
		t.Fatalf("unexpected pod count %d", got)
	}
}
