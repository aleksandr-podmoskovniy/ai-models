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

package publicationops

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileCreatesPublishPodForHuggingFaceOperation(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := huggingFaceRequest()
	operation := mustNewOperation(t, request)

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	result := mustReconcile(t, reconciler, operation)
	if result.RequeueAfter != 0 {
		t.Fatalf("expected pod watcher flow without immediate timer requeue, got %#v", result)
	}

	pod := mustGetSourceWorkerPod(t, kubeClient, request)
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("unexpected owner reference count %d", len(pod.OwnerReferences))
	}
	if got, want := pod.OwnerReferences[0].Kind, "ConfigMap"; got != want {
		t.Fatalf("unexpected owner kind %q", got)
	}

	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseRunning; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestReconcileCreatesPublishPodForHTTPOperation(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := httpRequest()
	operation := mustNewOperation(t, request)

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	result := mustReconcile(t, reconciler, operation)
	if result.RequeueAfter != 0 {
		t.Fatalf("expected pod watcher flow without immediate timer requeue, got %#v", result)
	}

	_ = mustGetSourceWorkerPod(t, kubeClient, request)
	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseRunning; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestReconcileProjectsHTTPAuthSecretForPublicationWorker(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := httpRequest()
	request.Spec.Source.HTTP.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "http-auth"}
	operation := mustNewOperation(t, request)

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1ObjectMeta("team-a", "http-auth"),
		Data: map[string][]byte{
			"authorization": []byte("Bearer abc"),
		},
	}

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation, sourceSecret)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseRunning; got != want {
		t.Fatalf("unexpected phase %q", got)
	}

	secretName, err := resourcenames.SourceWorkerAuthSecretName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerAuthSecretName() error = %v", err)
	}

	var projected corev1.Secret
	if err := kubeClient.Get(
		context.Background(),
		client.ObjectKey{Name: secretName, Namespace: "d8-ai-models"},
		&projected,
	); err != nil {
		t.Fatalf("Get(projected secret) error = %v", err)
	}
	if got, want := string(projected.Data["authorization"]), "Bearer abc"; got != want {
		t.Fatalf("unexpected projected authorization %q", got)
	}
}

func TestReconcileFailsWhenSourceAuthSecretIsMissing(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := httpRequest()
	request.Spec.Source.HTTP.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "http-auth"}
	operation := mustNewOperation(t, request)

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestReconcileMarksOperationSucceededFromWorkerResult(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := huggingFaceRequest()
	operation := mustNewOperation(t, request)
	mustSetRunning(t, operation, "ai-model-publish-1111-2222")
	operation.Data[workerResultDataKey] = sampleWorkerResultJSON()

	pod := sourceWorkerPod(t, request, corev1.PodSucceeded)
	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation, pod)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseSucceeded; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	result, err := ResultFromConfigMap(&updated)
	if err != nil {
		t.Fatalf("ResultFromConfigMap() error = %v", err)
	}
	if got, want := result.Snapshot.Artifact.URI, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/1111-2222@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected artifact URI %q", got)
	}
}

func TestReconcileFailsMalformedWorkerResult(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := huggingFaceRequest()
	operation := mustNewOperation(t, request)
	mustSetRunning(t, operation, "ai-model-publish-1111-2222")
	operation.Data[workerResultDataKey] = "{"

	pod := sourceWorkerPod(t, request, corev1.PodSucceeded)
	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation, pod)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestReconcileSourceWorkerAwaitingResultThenSucceedsOnReplay(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := huggingFaceRequest()
	operation := mustNewOperation(t, request)
	mustSetRunning(t, operation, "ai-model-publish-1111-2222")

	pod := sourceWorkerPod(t, request, corev1.PodSucceeded)
	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation, pod)
	first := mustReconcile(t, reconciler, operation)
	if first.RequeueAfter <= 0 {
		t.Fatalf("expected awaiting-result requeue, got %#v", first)
	}

	running := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&running).Phase, PhaseRunning; got != want {
		t.Fatalf("unexpected phase after awaiting-result %q", got)
	}

	running.Data[workerResultDataKey] = sampleWorkerResultJSON()
	if err := kubeClient.Update(context.Background(), &running); err != nil {
		t.Fatalf("Update(operation) error = %v", err)
	}

	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseSucceeded; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func sourceWorkerPod(t *testing.T, request publicationports.Request, phase corev1.PodPhase) *corev1.Pod {
	t.Helper()

	podName, err := resourcenames.SourceWorkerPodName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerPodName() error = %v", err)
	}

	return &corev1.Pod{
		ObjectMeta: metav1ObjectMeta("d8-ai-models", podName),
		Status: corev1.PodStatus{
			Phase: phase,
		},
	}
}

func mustGetSourceWorkerPod(t *testing.T, kubeClient client.Client, request publicationports.Request) corev1.Pod {
	t.Helper()

	podName, err := resourcenames.SourceWorkerPodName(request.Owner.UID)
	if err != nil {
		t.Fatalf("SourceWorkerPodName() error = %v", err)
	}

	var pod corev1.Pod
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: "d8-ai-models"}, &pod); err != nil {
		t.Fatalf("Get(pod) error = %v", err)
	}

	return pod
}
