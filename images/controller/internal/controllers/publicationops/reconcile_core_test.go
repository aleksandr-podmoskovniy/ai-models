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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconcileIgnoresUnmanagedConfigMap(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	operation := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plain-configmap",
			Namespace: "d8-ai-models",
		},
	}

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	stored := mustGetConfigMap(t, kubeClient, operation)
	if phase := StatusFromConfigMap(&stored).Phase; phase != PhasePending {
		t.Fatalf("unexpected phase %q", phase)
	}
}

func TestReconcileMarksMalformedRequestAsFailed(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	operation := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ai-model-publication-bad",
			Namespace: "d8-ai-models",
			Labels: map[string]string{
				managedLabelKey: managedLabelValue,
			},
			Annotations: map[string]string{
				phaseAnnotationKey: string(PhasePending),
			},
		},
		Data: map[string]string{
			requestDataKey: "{",
		},
	}

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&updated).Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestReconcileSkipsTerminalOperation(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	request := huggingFaceRequest()
	operation := mustNewOperation(t, request)
	if err := SetFailed(operation, "already failed"); err != nil {
		t.Fatalf("SetFailed() error = %v", err)
	}

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	stored := mustGetConfigMap(t, kubeClient, operation)
	if got, want := StatusFromConfigMap(&stored).Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestReconcileFailsSucceededOperationWithoutPersistedResult(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	operation := mustNewOperation(t, huggingFaceRequest())
	operation.Annotations[phaseAnnotationKey] = string(PhaseSucceeded)

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	status := StatusFromConfigMap(&updated)
	if got, want := status.Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if status.Message == "" {
		t.Fatal("expected corruption message")
	}
}

func TestReconcileFailsUnknownPersistedPhase(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	operation := mustNewOperation(t, huggingFaceRequest())
	operation.Annotations[phaseAnnotationKey] = "Corrupted"

	reconciler, kubeClient := newPublicationOperationReconciler(t, scheme, operation)
	mustReconcile(t, reconciler, operation)

	updated := mustGetConfigMap(t, kubeClient, operation)
	status := StatusFromConfigMap(&updated)
	if got, want := status.Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if status.Message == "" {
		t.Fatal("expected corruption message")
	}
}

func TestReconcileFailsRunningOperationWithMalformedPersistedUploadPayload(t *testing.T) {
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
		t.Fatal("expected corruption message")
	}
}
