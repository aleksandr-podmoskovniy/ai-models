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

package catalogcleanup

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestModelReconcilerEnqueuesGarbageCollectionRequestAndRemovesFinalizerAfterCompletedCleanup(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	const sessionToken = "session-token-1"
	stateSecret := sourceWorkerStateSecretWithSessionToken(t, "d8-ai-models", model.GetUID(), sessionToken)
	cleaner := &recordingCleaner{}
	reconciler, kubeClient := newModelReconcilerWithCleaner(t, cleaner, model, stateSecret)
	if err := reconciler.cleanupState.MarkCompleted(context.Background(), model); err != nil {
		t.Fatalf("MarkCompleted() error = %v", err)
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected result %#v", result)
	}
	if got := len(cleaner.calls); got != 0 {
		t.Fatalf("completed cleanup was repeated %d time(s)", got)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) error = %v", err)
		}
	} else if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected finalizer to be removed once garbage collection is enqueued")
	}

	var requestSecret corev1.Secret
	key := client.ObjectKey{Namespace: "d8-ai-models", Name: dmcrGCRequestSecretName(model.GetUID())}
	if err := kubeClient.Get(context.Background(), key, &requestSecret); err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if requestSecret.Annotations[dmcrGCRequestedAnnotationKey] == "" {
		t.Fatalf("expected queued request annotation on garbage-collection request secret, got %#v", requestSecret.Annotations)
	}
	if requestSecret.Annotations[dmcrGCSwitchAnnotationKey] != "" {
		t.Fatalf("expected delete-triggered garbage-collection request to stay queued, got %#v", requestSecret.Annotations)
	}
	if got, want := string(requestSecret.Data[dmcrGCDirectUploadTokenKey]), sessionToken; got != want {
		t.Fatalf("expected delete-triggered garbage-collection request to snapshot current direct-upload session token %q, got %q", want, got)
	}
	stateSecretKey := client.ObjectKey{Namespace: "d8-ai-models", Name: stateSecret.Name}
	if err := kubeClient.Get(context.Background(), stateSecretKey, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected publication state secret to be deleted once delete finalizer is removed, got err=%v", err)
	}
}

func TestModelReconcilerRemovesFinalizerWhenQueuedGarbageCollectionRequestAlreadyExists(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	gcSecret := requestedGCSecret("d8-ai-models", model.GetUID())
	cleaner := &recordingCleaner{}
	reconciler, kubeClient := newModelReconcilerWithCleaner(t, cleaner, model, gcSecret)
	if err := reconciler.cleanupState.MarkCompleted(context.Background(), model); err != nil {
		t.Fatalf("MarkCompleted() error = %v", err)
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected result %#v", result)
	}
	if got := len(cleaner.calls); got != 0 {
		t.Fatalf("completed cleanup was repeated %d time(s)", got)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) error = %v", err)
		}
	} else if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected finalizer to be removed when queued garbage-collection request already exists")
	}
}
