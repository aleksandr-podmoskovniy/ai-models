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
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestModelReconcilerMarksDeletingStatusOnInvalidCleanupHandle(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	model.Annotations = map[string]string{cleanuphandle.AnnotationKey: "{not-json"}
	reconciler, kubeClient := newModelReconciler(t, model)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("unexpected result %#v", result)
	}

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonFailed)
}

func TestModelReconcilerCreatesCleanupJobOnDelete(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	reconciler, kubeClient := newModelReconciler(t, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	jobName := cleanupJobName(t, model)
	assertCleanupJobExists(t, kubeClient, jobName)
	registrySecretName, err := resourcenames.OCIRegistryAuthSecretName(model.GetUID())
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: registrySecretName, Namespace: "d8-ai-models"}, &corev1.Secret{}); err != nil {
		t.Fatalf("expected projected OCI auth secret, got err=%v", err)
	}
	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonPending)
}

func TestModelReconcilerKeepsPendingStatusWhileCleanupJobRuns(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	jobName := cleanupJobName(t, model)
	reconciler, kubeClient := newModelReconciler(t, model, runningJob("d8-ai-models", jobName))

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("unexpected result %#v", result)
	}

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonPending)
}

func TestModelReconcilerFailsClosedWhenCleanupJobFails(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	jobName := cleanupJobName(t, model)
	reconciler, kubeClient := newModelReconciler(t, model, failedJob("d8-ai-models", jobName))

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("unexpected result %#v", result)
	}

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonFailed)
}

func TestModelReconcilerMarksInvalidCleanupHandleAsFailedOnDelete(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	model.Annotations = map[string]string{cleanuphandle.AnnotationKey: `{"kind":"BackendArtifact","backend":{}}`}
	reconciler, kubeClient := newModelReconciler(t, model)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("unexpected requeue %#v", result)
	}

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonFailed)
}

func TestModelReconcilerDeletesInFlightPublicationRuntimeBeforeRemovingFinalizer(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()

	podName, err := resourcenames.SourceWorkerPodName(model.GetUID())
	if err != nil {
		t.Fatalf("SourceWorkerPodName() error = %v", err)
	}
	stateSecretName, err := resourcenames.SourceWorkerStateSecretName(model.GetUID())
	if err != nil {
		t.Fatalf("SourceWorkerStateSecretName() error = %v", err)
	}
	authSecretName, err := resourcenames.OCIRegistryAuthSecretName(model.GetUID())
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	caSecretName, err := resourcenames.OCIRegistryCASecretName(model.GetUID())
	if err != nil {
		t.Fatalf("OCIRegistryCASecretName() error = %v", err)
	}

	reconciler, kubeClient := newModelReconciler(
		t,
		model,
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: "d8-ai-models"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: stateSecretName, Namespace: "d8-ai-models"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: "d8-ai-models"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: "d8-ai-models"}},
	)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() first pass error = %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("unexpected first requeue %#v", result)
	}

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonPending)

	for _, key := range []client.ObjectKey{
		{Name: podName, Namespace: "d8-ai-models"},
		{Name: stateSecretName, Namespace: "d8-ai-models"},
		{Name: authSecretName, Namespace: "d8-ai-models"},
		{Name: caSecretName, Namespace: "d8-ai-models"},
	} {
		if err := kubeClient.Get(context.Background(), key, &corev1.Secret{}); err != nil && !apierrors.IsNotFound(err) && key.Name != podName {
			t.Fatalf("expected resource %s/%s to be deleted, got err=%v", key.Namespace, key.Name, err)
		}
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: "d8-ai-models"}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected publish worker pod to be deleted, got err=%v", err)
	}

	var stillDeleting modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &stillDeleting); err != nil {
		t.Fatalf("Get(model) after runtime cleanup error = %v", err)
	}
	if !controllerutil.ContainsFinalizer(&stillDeleting, Finalizer) {
		t.Fatal("expected finalizer to remain until the next delete reconciliation confirms runtime cleanup")
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() second pass error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) after finalizer removal error = %v", err)
		}
		return
	}
	if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatalf("expected finalizer to be removed after runtime resources were cleaned up, got %#v", updated.Finalizers)
	}
}
