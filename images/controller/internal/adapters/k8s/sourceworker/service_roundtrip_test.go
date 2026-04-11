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

	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceRoundTripGetOrCreateAndDelete(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			owner,
			testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
		).
		Build()

	runtime, err := NewService(kubeClient, scheme, testOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	handle, created, err := runtime.GetOrCreate(context.Background(), owner, testOperationRequest())
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created || handle == nil || handle.Name == "" {
		t.Fatalf("unexpected source worker handle %#v created=%v", handle, created)
	}

	var pod corev1.Pod
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: handle.Name, Namespace: "d8-ai-models"}, &pod); err != nil {
		t.Fatalf("Get(pod) error = %v", err)
	}

	if err := handle.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(&pod), &pod); !apierrors.IsNotFound(err) {
		t.Fatalf("expected pod to be deleted, got err=%v", err)
	}
	registrySecretName, err := resourcenames.OCIRegistryAuthSecretName(owner.UID)
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: registrySecretName, Namespace: "d8-ai-models"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected OCI auth secret to be deleted, got err=%v", err)
	}
}
