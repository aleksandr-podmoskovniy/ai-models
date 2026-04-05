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
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileCreatesUploadSessionSupplementsForUploadOperation(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := uploadRequest(modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory)
	operation := mustNewOperation(t, request)

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	for _, object := range uploadSessionObjects(t, request, time.Now().UTC().Add(15*time.Minute)) {
		switch obj := object.(type) {
		case *corev1.Pod:
			var stored corev1.Pod
			if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(obj), &stored); err != nil {
				t.Fatalf("expected upload session object %T to be created: %v", obj, err)
			}
		case *corev1.Service:
			var stored corev1.Service
			if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(obj), &stored); err != nil {
				t.Fatalf("expected upload session object %T to be created: %v", obj, err)
			}
		case *corev1.Secret:
			var stored corev1.Secret
			if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(obj), &stored); err != nil {
				t.Fatalf("expected upload session object %T to be created: %v", obj, err)
			}
		}
	}
}

func TestReconcileSetsUploadReadyForRunningUploadSession(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := uploadRequest(modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory)
	operation := mustNewOperation(t, request)
	objects := uploadSessionObjects(t, request, time.Now().UTC().Add(15*time.Minute))

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, append([]client.Object{operation}, objects...)...)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	upload, err := UploadStatusFromConfigMap(&updated)
	if err != nil {
		t.Fatalf("UploadStatusFromConfigMap() error = %v", err)
	}
	if upload == nil || upload.Command == "" {
		t.Fatalf("unexpected upload status %#v", upload)
	}
}

func TestReconcileFailsMalformedPersistedUploadState(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := uploadRequest(modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory)
	operation := mustNewOperation(t, request)
	mustSetRunning(t, operation, "ai-model-upload-1111-2224")
	operation.Data[uploadDataKey] = `{}`

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	status := StatusFromConfigMap(&updated)
	if got, want := status.Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if status.Message == "" {
		t.Fatal("expected failure message for malformed persisted upload")
	}
}

func TestReconcileRunningUploadSessionDoesNotRewriteIdenticalUploadStatus(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := uploadRequest(modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory)
	operation := mustNewOperation(t, request)
	objects := uploadSessionObjects(t, request, time.Now().UTC().Add(15*time.Minute))

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, append([]client.Object{operation}, objects...)...)
	first := mustReconcile(t, reconciler, operation)
	if first.RequeueAfter <= 0 {
		t.Fatalf("expected positive requeue after first upload reconcile, got %#v", first)
	}

	initial := mustGetConfigMap(t, kubeClient, operation)
	initialUpload := initial.Data[uploadDataKey]
	initialWorker := initial.Annotations[workerAnnotationKey]

	second := mustReconcile(t, reconciler, operation)
	if second.RequeueAfter <= 0 {
		t.Fatalf("expected positive requeue after replay, got %#v", second)
	}

	replayed := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&replayed).Phase, PhaseRunning; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if got := replayed.Data[uploadDataKey]; got != initialUpload {
		t.Fatalf("upload payload changed on identical replay: %q != %q", got, initialUpload)
	}
	if got := replayed.Annotations[workerAnnotationKey]; got != initialWorker {
		t.Fatalf("worker annotation changed on identical replay: %q != %q", got, initialWorker)
	}
}

func TestReconcileExpiredUploadSessionFailsOnceAndStaysTerminalOnReplay(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := uploadRequest(modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory)
	operation := mustNewOperation(t, request)
	objects := uploadSessionObjects(t, request, time.Now().UTC().Add(-1*time.Minute))

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, append([]client.Object{operation}, objects...)...)
	mustReconcile(t, reconciler, operation)

	failed := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&failed).Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase after expiry %q", got)
	}

	mustReconcile(t, reconciler, operation)

	replayed := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&replayed).Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected replayed phase %q", got)
	}
	if _, found := replayed.Data[uploadDataKey]; found {
		t.Fatal("upload payload must be cleared after terminal expiry failure")
	}
}

func TestReconcileFailsUploadModelKitOnCurrentBackend(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := uploadRequest(modelsv1alpha1.ModelUploadFormatModelKit)
	operation := mustNewOperation(t, request)

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func uploadSessionObjects(t *testing.T, request publicationports.Request, expiresAt time.Time) []client.Object {
	t.Helper()

	podName, err := resourcenames.UploadSessionPodName(request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionPodName() error = %v", err)
	}
	serviceName, err := resourcenames.UploadSessionServiceName(request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionServiceName() error = %v", err)
	}
	secretName, err := resourcenames.UploadSessionSecretName(request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionSecretName() error = %v", err)
	}

	return []client.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: "d8-ai-models",
				UID:       types.UID("upload-secret-uid"),
				Annotations: map[string]string{
					"ai-models.deckhouse.io/upload-expires-at": expiresAt.UTC().Format(time.RFC3339),
				},
			},
			Data: map[string][]byte{
				"token": []byte("deadbeef"),
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: "d8-ai-models",
				UID:       types.UID("upload-service-uid"),
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: "d8-ai-models",
				UID:       types.UID("upload-pod-uid"),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Args: []string{
						"--artifact-uri", "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1-upload/1111-2224@sha256:deadbeef",
					},
				}},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		},
	}
}
