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
	"encoding/json"
	"testing"

	directuploadstate "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/directuploadstate"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceGetOrCreateRecreatesFailedPodWhenDirectUploadIsRunning(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	request := testOperationRequest()
	owner := testkit.NewModel()
	owner.UID = request.Owner.UID
	owner.Name = request.Owner.Name

	podName, err := resourcenames.SourceWorkerPodName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerPodName() error = %v", err)
	}
	stateSecretName, err := resourcenames.SourceWorkerStateSecretName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerStateSecretName() error = %v", err)
	}
	stateSecret, err := directuploadstate.NewSecret(directuploadstate.SecretSpec{
		Name:            stateSecretName,
		Namespace:       "d8-ai-models",
		OwnerGeneration: 1,
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}

	directState, err := directuploadstate.StateFromSecret(stateSecret)
	if err != nil {
		t.Fatalf("StateFromSecret() error = %v", err)
	}
	directState.Phase = modelpackports.DirectUploadStatePhaseRunning
	directState.CurrentLayer = &modelpackports.DirectUploadCurrentLayer{
		Key:               "model|application/test",
		SessionToken:      "session-1",
		PartSizeBytes:     64,
		TotalSizeBytes:    256,
		UploadedSizeBytes: 128,
	}
	payload, err := json.Marshal(directState)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	stateSecret.Data["state.json"] = payload

	failedPod := &corev1.Pod{
		ObjectMeta: testkit.NewModel().ObjectMeta,
	}
	failedPod.Name = podName
	failedPod.Namespace = "d8-ai-models"
	failedPod.Labels = buildLabels(request.Owner)
	failedPod.Status.Phase = corev1.PodFailed

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			owner,
			failedPod,
			stateSecret,
			testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
		).
		Build()

	service, err := NewService(kubeClient, scheme, testOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected failed worker pod to be recreated")
	}
	if handle == nil || handle.Name != podName {
		t.Fatalf("unexpected source worker handle %#v", handle)
	}

	var recreated corev1.Pod
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: "d8-ai-models"}, &recreated); err != nil {
		t.Fatalf("Get(recreated pod) error = %v", err)
	}
	if recreated.Status.Phase == corev1.PodFailed {
		t.Fatal("expected recreated pod to replace failed instance")
	}
}

func TestServiceGetOrCreateRecreatesInterruptedFailedPodBeforeDirectUploadStarts(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	request := testOperationRequest()
	owner := testkit.NewModel()
	owner.UID = request.Owner.UID
	owner.Name = request.Owner.Name

	podName, err := resourcenames.SourceWorkerPodName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerPodName() error = %v", err)
	}
	stateSecretName, err := resourcenames.SourceWorkerStateSecretName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerStateSecretName() error = %v", err)
	}
	stateSecret, err := directuploadstate.NewSecret(directuploadstate.SecretSpec{
		Name:            stateSecretName,
		Namespace:       "d8-ai-models",
		OwnerGeneration: 1,
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}

	failedPod := &corev1.Pod{
		ObjectMeta: testkit.NewModel().ObjectMeta,
	}
	failedPod.Name = podName
	failedPod.Namespace = "d8-ai-models"
	failedPod.Labels = buildLabels(request.Owner)
	failedPod.Status.Phase = corev1.PodFailed
	failedPod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "publish",
		State: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				Message: "Get \"https://huggingface.co/api/models/Qwen/Qwen2.5-0.5B-Instruct\": context canceled",
			},
		},
	}}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			owner,
			failedPod,
			stateSecret,
			testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
		).
		Build()

	service, err := NewService(kubeClient, scheme, testOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected interrupted failed worker pod to be recreated")
	}
	if handle == nil || handle.Name != podName {
		t.Fatalf("unexpected source worker handle %#v", handle)
	}

	var recreated corev1.Pod
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: "d8-ai-models"}, &recreated); err != nil {
		t.Fatalf("Get(recreated pod) error = %v", err)
	}
	if recreated.Status.Phase == corev1.PodFailed {
		t.Fatal("expected recreated pod to replace interrupted failed instance")
	}
}
