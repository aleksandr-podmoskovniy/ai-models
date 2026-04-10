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
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetOrCreateReusesExistingSession(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("3333-4444")

	request := testUploadOperationContext()
	request.Request.Owner.UID = types.UID("3333-4444")

	pod, err := BuildPod(request, uploadOptions(), "ai-model-upload-auth-3333-4444")
	if err != nil {
		t.Fatalf("BuildPod() error = %v", err)
	}
	pod.Status.Phase = corev1.PodRunning

	serviceName, err := resourcenames.UploadSessionServiceName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionServiceName() error = %v", err)
	}
	secretName, err := resourcenames.UploadSessionSecretName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionSecretName() error = %v", err)
	}
	ingressName, err := resourcenames.UploadSessionIngressName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionIngressName() error = %v", err)
	}
	expiresAt := metav1.NewTime(time.Now().Add(10 * time.Minute).UTC())

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			owner,
			testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
			pod,
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: "d8-ai-models"}},
			&networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: ingressName, Namespace: "d8-ai-models"},
				Spec: networkingv1.IngressSpec{
					TLS: []networkingv1.IngressTLS{{Hosts: []string{"ai-models.example.com"}, SecretName: "ingress-tls"}},
					Rules: []networkingv1.IngressRule{{
						Host: "ai-models.example.com",
					}},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        secretName,
					Namespace:   "d8-ai-models",
					Annotations: map[string]string{"ai-models.deckhouse.io/upload-expires-at": expiresAt.Format(time.RFC3339)},
				},
				Data: map[string][]byte{"token": []byte("existing-token")},
			},
		).
		Build()

	service, err := NewService(kubeClient, scheme, uploadOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if created {
		t.Fatal("expected existing upload session to be reused")
	}
	if handle == nil || handle.UploadStatus.Repository == "" || handle.UploadStatus.ExpiresAt == nil {
		t.Fatalf("unexpected reused session %#v", handle)
	}
	if handle.UploadStatus.ExternalURL == "" || handle.UploadStatus.InClusterURL == "" {
		t.Fatalf("expected upload URLs in reused session %#v", handle.UploadStatus)
	}
}

func TestGetOrCreateRecoversFromPartialAlreadyExists(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("3333-4444")

	request := testUploadOperationContext()
	request.Request.Owner.UID = types.UID("3333-4444")

	serviceName, err := resourcenames.UploadSessionServiceName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionServiceName() error = %v", err)
	}
	secretName, err := resourcenames.UploadSessionSecretName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionSecretName() error = %v", err)
	}
	ingressName, err := resourcenames.UploadSessionIngressName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionIngressName() error = %v", err)
	}
	expiresAt := metav1.NewTime(time.Now().Add(10 * time.Minute).UTC())

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			owner,
			testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: "d8-ai-models"}},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        secretName,
					Namespace:   "d8-ai-models",
					Annotations: map[string]string{"ai-models.deckhouse.io/upload-expires-at": expiresAt.Format(time.RFC3339)},
				},
				Data: map[string][]byte{"token": []byte("existing-token")},
			},
		).
		Build()

	service, err := NewService(kubeClient, scheme, uploadOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected partial replay to create the missing pod")
	}
	if handle == nil || handle.WorkerName == "" {
		t.Fatalf("unexpected session %#v", handle)
	}

	for _, object := range []client.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: handle.WorkerName, Namespace: "d8-ai-models"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: "d8-ai-models"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "d8-ai-models"}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: ingressName, Namespace: "d8-ai-models"}},
	} {
		stored := object.DeepCopyObject().(client.Object)
		if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(object), stored); err != nil {
			t.Fatalf("Get(%T) error = %v", object, err)
		}
	}
}

var _ publicationports.UploadSessionRuntime = (*Service)(nil)

func uploadOptions() Options {
	return Options{
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
		TokenTTL: 15 * time.Minute,
	}
}
