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

package uploadsession

import (
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceRoundTripGetOrCreateAndDelete(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			owner,
			testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
		).
		Build()

	runtime, err := NewService(kubeClient, scheme, Options{
		Runtime: workloadpod.RuntimeOptions{
			Namespace:             "d8-ai-models",
			Image:                 "backend:latest",
			ServiceAccountName:    "ai-models-controller",
			OCIRepositoryPrefix:   "registry.internal.local/ai-models",
			OCIRegistrySecretName: "ai-models-dmcr-auth-write",
			ObjectStorage: objectstorage.Options{
				Bucket:                "ai-models",
				EndpointURL:           "https://s3.example.com",
				Region:                "us-east-1",
				UsePathStyle:          true,
				CredentialsSecretName: "ai-models-artifacts",
			},
		},
		Ingress: IngressOptions{
			Host:          "ai-models.example.com",
			ClassName:     "nginx",
			TLSSecretName: "ingress-tls",
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	request := testUploadOperationContext()
	request.Request.Owner.UID = owner.UID
	request.Request.Owner.Name = owner.Name
	request.Request.Identity.Name = owner.Name

	handle, created, err := runtime.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created || handle == nil || handle.WorkerName == "" {
		t.Fatalf("unexpected upload session handle %#v created=%v", handle, created)
	}
	if handle.UploadStatus.InClusterURL == "" || handle.UploadStatus.Repository == "" {
		t.Fatalf("unexpected upload session status %#v", handle.UploadStatus)
	}

	var pod corev1.Pod
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: handle.WorkerName, Namespace: "d8-ai-models"}, &pod); err != nil {
		t.Fatalf("Get(pod) error = %v", err)
	}
	ingressName, err := resourcenames.UploadSessionIngressName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionIngressName() error = %v", err)
	}
	var ingress networkingv1.Ingress
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: ingressName, Namespace: "d8-ai-models"}, &ingress); err != nil {
		t.Fatalf("Get(ingress) error = %v", err)
	}

	if err := handle.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(&pod), &pod); !apierrors.IsNotFound(err) {
		t.Fatalf("expected pod to be deleted, got err=%v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(&ingress), &ingress); !apierrors.IsNotFound(err) {
		t.Fatalf("expected ingress to be deleted, got err=%v", err)
	}
}
