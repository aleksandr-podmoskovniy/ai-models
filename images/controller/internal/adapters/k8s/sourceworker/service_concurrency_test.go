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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
