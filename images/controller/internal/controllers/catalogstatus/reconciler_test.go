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

package catalogstatus

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestModelReconcilerProjectsPublishingStatusFromRunningSourceWorker(t *testing.T) {
	t.Parallel()

	model := testModel()
	sourceWorkers := &fakeSourceWorkerRuntime{handle: runningSourceWorkerHandle()}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, &fakeUploadSessionRuntime{}, model)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter <= 0 {
		t.Fatalf("expected requeue while publication is running, got %#v", result)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := updated.Status.Phase, modelsv1alpha1.ModelPhasePublishing; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if updated.Status.Source == nil || updated.Status.Source.ResolvedType != modelsv1alpha1.ModelSourceTypeHuggingFace {
		t.Fatalf("unexpected source status %#v", updated.Status.Source)
	}
}

func TestClusterModelReconcilerProjectsClusterScopedRunningStatus(t *testing.T) {
	t.Parallel()

	clusterModel := testClusterModel()
	sourceWorkers := &fakeSourceWorkerRuntime{handle: runningSourceWorkerHandle()}
	reconciler, kubeClient := newClusterModelReconciler(t, sourceWorkers, &fakeUploadSessionRuntime{}, clusterModel)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(clusterModel),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.ClusterModel
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(clusterModel), &updated); err != nil {
		t.Fatalf("Get(clusterModel) error = %v", err)
	}
	if updated.Status.Source == nil || updated.Status.Source.ResolvedType != modelsv1alpha1.ModelSourceTypeHuggingFace {
		t.Fatalf("unexpected source status %#v", updated.Status.Source)
	}
}

func TestModelReconcilerPublishesReadyStatusFromSucceededWorker(t *testing.T) {
	t.Parallel()

	model := testModel()
	deleted := false
	sourceWorkers := &fakeSourceWorkerRuntime{handle: succeededSourceWorkerHandle(t, &deleted)}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, &fakeUploadSessionRuntime{}, model)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	})
	if err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	if !result.Requeue {
		t.Fatalf("expected requeue after writing cleanup handle, got %#v", result)
	}

	var annotated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &annotated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if _, found, err := cleanuphandle.FromObject(&annotated); err != nil || !found {
		t.Fatalf("expected cleanup handle annotation after first reconcile, found=%v err=%v", found, err)
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	var ready modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &ready); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := ready.Status.Phase, modelsv1alpha1.ModelPhaseReady; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if ready.Status.Artifact == nil || ready.Status.Artifact.URI != "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef" {
		t.Fatalf("unexpected artifact status %#v", ready.Status.Artifact)
	}
	if ready.Status.Resolved == nil || ready.Status.Resolved.SourceRepoID != "deepseek-ai/DeepSeek-R1" {
		t.Fatalf("unexpected resolved status %#v", ready.Status.Resolved)
	}
	if !deleted {
		t.Fatal("expected worker delete callback to run after successful projection")
	}
}

func TestModelReconcilerMarksFailureFromFailedWorker(t *testing.T) {
	t.Parallel()

	model := testModel()
	sourceWorkers := &fakeSourceWorkerRuntime{handle: failedSourceWorkerHandle("hf import failed")}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, &fakeUploadSessionRuntime{}, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var failed modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &failed); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := failed.Status.Phase, modelsv1alpha1.ModelPhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestModelReconcilerProjectsWaitForUploadStatus(t *testing.T) {
	t.Parallel()

	model := testUploadModel()
	uploadSessions := &fakeUploadSessionRuntime{handle: runningUploadSessionHandle()}
	reconciler, kubeClient := newModelReconciler(t, &fakeSourceWorkerRuntime{}, uploadSessions, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := updated.Status.Phase, modelsv1alpha1.ModelPhaseWaitForUpload; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if updated.Status.Upload == nil || updated.Status.Upload.Command != "curl -T file" {
		t.Fatalf("unexpected upload status %#v", updated.Status.Upload)
	}
}
