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
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestModelReconcilerDoesNotAddFinalizerWithoutCleanupHandle(t *testing.T) {
	t.Parallel()

	model := testModel()
	reconciler, kubeClient := newModelReconciler(t, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("did not expect cleanup finalizer without cleanup handle")
	}
}

func TestClusterModelReconcilerAddsFinalizerWhenCleanupHandleExists(t *testing.T) {
	t.Parallel()

	model := testClusterModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/cluster/mixtral-8x7b@sha256:deadbeef")
	reconciler, kubeClient := newClusterModelReconciler(t, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.ClusterModel
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected cleanup finalizer")
	}
}

func TestModelReconcilerRemovesFinalizerOnDeleteWithoutCleanupHandle(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	reconciler, kubeClient := newModelReconciler(t, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) error = %v", err)
		}
		return
	}
	if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected stale finalizer to be removed")
	}
}

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

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonCleanupFailed)
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
	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonCleanupPending)
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

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonCleanupPending)
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

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonCleanupFailed)
}

func TestModelReconcilerRemovesFinalizerAfterCompletedJob(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	jobName := cleanupJobName(t, model)
	reconciler, kubeClient := newModelReconciler(t, model, completedJob("d8-ai-models", jobName))

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) error = %v", err)
		}
		return
	}
	if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected finalizer to be removed after completed cleanup job")
	}
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

	assertCleanupCondition(t, kubeClient, model, modelsv1alpha1.ModelPhaseDeleting, modelsv1alpha1.ModelConditionReasonCleanupFailed)
}

func assertCleanupJobExists(t *testing.T, kubeClient client.Client, jobName string) {
	t.Helper()

	var job batchv1.Job
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: jobName}, &job); err != nil {
		t.Fatalf("Get(job) error = %v", err)
	}
	if got, want := job.Labels["ai-models.deckhouse.io/owner-kind"], modelsv1alpha1.ModelKind; got != want {
		t.Fatalf("unexpected owner-kind label %q", got)
	}
}

func assertCleanupCondition(
	t *testing.T,
	kubeClient client.Client,
	model *modelsv1alpha1.Model,
	phase modelsv1alpha1.ModelPhase,
	reason modelsv1alpha1.ModelConditionReason,
) {
	t.Helper()

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if updated.Status.Phase != phase {
		t.Fatalf("unexpected phase %q", updated.Status.Phase)
	}

	condition := apimeta.FindStatusCondition(updated.Status.Conditions, string(modelsv1alpha1.ModelConditionCleanupCompleted))
	if condition == nil || condition.Reason != string(reason) {
		t.Fatalf("unexpected cleanup condition %#v", condition)
	}
}
