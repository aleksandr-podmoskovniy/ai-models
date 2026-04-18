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
