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
	auditapp "github.com/deckhouse/ai-models/controller/internal/application/publishaudit"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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
	if deleted {
		t.Fatal("worker must not be deleted before ready status is persisted")
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
	if ready.Status.Resolved == nil {
		t.Fatalf("unexpected resolved status %#v", ready.Status.Resolved)
	}
	if !deleted {
		t.Fatal("expected worker delete callback to run after successful projection")
	}
}

func TestModelReconcilerRecordsPublicationSucceededAuditEvent(t *testing.T) {
	t.Parallel()

	model := testModel()
	deleted := false
	auditSink := &fakeAuditSink{}
	sourceWorkers := &fakeSourceWorkerRuntime{handle: succeededSourceWorkerHandle(t, &deleted)}
	reconciler, _ := newModelReconcilerWithSink(t, sourceWorkers, &fakeUploadSessionRuntime{}, auditSink, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	for _, record := range auditSink.records {
		if record.Reason == auditapp.ReasonPublicationSuccess {
			return
		}
	}
	t.Fatalf("expected %s event in %#v", auditapp.ReasonPublicationSuccess, auditSink.records)
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

func TestModelReconcilerMarksLegacyUnsupportedRemoteSourceFailedWithoutRuntime(t *testing.T) {
	t.Parallel()

	model := testModel()
	model.Spec.Source.URL = "https://downloads.example.com/model.tar.gz"

	sourceWorkers := &fakeSourceWorkerRuntime{}
	uploadSessions := &fakeUploadSessionRuntime{}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, uploadSessions, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if sourceWorkers.calls != 0 {
		t.Fatalf("legacy unsupported source must not create source worker, got %d calls", sourceWorkers.calls)
	}
	if uploadSessions.calls != 0 {
		t.Fatalf("legacy unsupported source must not create upload session, got %d calls", uploadSessions.calls)
	}

	var failed modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &failed); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := failed.Status.Phase, modelsv1alpha1.ModelPhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if failed.Status.Source != nil {
		t.Fatalf("legacy unsupported source must not project resolvedType, got %#v", failed.Status.Source)
	}
	artifactPublished := apimeta.FindStatusCondition(failed.Status.Conditions, string(modelsv1alpha1.ModelConditionArtifactPublished))
	if artifactPublished == nil || artifactPublished.Reason != string(modelsv1alpha1.ModelConditionReasonUnsupportedSource) {
		t.Fatalf("unexpected artifact published condition %#v", artifactPublished)
	}
	ready := apimeta.FindStatusCondition(failed.Status.Conditions, string(modelsv1alpha1.ModelConditionReady))
	if ready == nil || ready.Reason != string(modelsv1alpha1.ModelConditionReasonUnsupportedSource) {
		t.Fatalf("unexpected ready condition %#v", ready)
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
	if updated.Status.Upload == nil || updated.Status.Upload.InClusterURL != "http://upload-worker.d8-ai-models.svc:8444/upload/token" {
		t.Fatalf("unexpected upload status %#v", updated.Status.Upload)
	}
}

func TestModelReconcilerKeepsUploadSessionAndMarksPublishingOnRawStageHandoff(t *testing.T) {
	t.Parallel()

	model := testUploadModel()
	deleted := false
	uploadSessions := &fakeUploadSessionRuntime{handle: succeededUploadSessionHandle(t, &deleted)}
	reconciler, kubeClient := newModelReconciler(t, &fakeSourceWorkerRuntime{}, uploadSessions, model)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.Requeue {
		t.Fatalf("expected requeue after persisting raw stage handle, got %#v", result)
	}
	if deleted {
		t.Fatal("upload session must not be deleted during raw-stage handoff")
	}
	if uploadSessions.markPublishingCalls != 1 {
		t.Fatalf("expected publishing session phase sync, got %d", uploadSessions.markPublishingCalls)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	handle, found, err := cleanuphandle.FromObject(&updated)
	if err != nil || !found {
		t.Fatalf("expected upload cleanup handle annotation, found=%v err=%v", found, err)
	}
	if handle.Kind != cleanuphandle.KindUploadStaging || handle.UploadStaging == nil {
		t.Fatalf("unexpected cleanup handle %#v", handle)
	}
}

func TestModelReconcilerMarksCompletedUploadSessionAfterReadyProjection(t *testing.T) {
	t.Parallel()

	model := testUploadModel()
	if err := cleanuphandle.SetOnObject(model, cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "ai-models",
			Key:       "raw/1111-2222/model.gguf",
			FileName:  "model.gguf",
			SizeBytes: 128,
		},
	}); err != nil {
		t.Fatalf("SetOnObject() error = %v", err)
	}
	model.Status = modelsv1alpha1.ModelStatus{
		ObservedGeneration: model.GetGeneration(),
		Phase:              modelsv1alpha1.ModelPhasePublishing,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: modelsv1alpha1.ModelSourceTypeUpload,
		},
	}

	deleted := false
	sourceWorkers := &fakeSourceWorkerRuntime{handle: succeededSourceWorkerHandle(t, &deleted)}
	uploadSessions := &fakeUploadSessionRuntime{}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, uploadSessions, model)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	})
	if err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	if !result.Requeue {
		t.Fatalf("expected requeue after backend cleanup handle handoff, got %#v", result)
	}
	if deleted {
		t.Fatal("source worker must not be deleted before ready status is persisted")
	}
	if uploadSessions.markCompletedCalls != 0 {
		t.Fatalf("upload session must not be marked completed before ready projection, got %d", uploadSessions.markCompletedCalls)
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}
	if uploadSessions.markCompletedCalls != 1 {
		t.Fatalf("expected completed session phase sync, got %d", uploadSessions.markCompletedCalls)
	}
	if deleted != true {
		t.Fatal("expected worker delete callback after ready projection")
	}

	var ready modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &ready); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := ready.Status.Phase, modelsv1alpha1.ModelPhaseReady; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestModelReconcilerMarksFailedUploadSessionWhenUploadPublishFails(t *testing.T) {
	t.Parallel()

	model := testUploadModel()
	if err := cleanuphandle.SetOnObject(model, cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "ai-models",
			Key:       "raw/1111-2222/model.gguf",
			FileName:  "model.gguf",
			SizeBytes: 128,
		},
	}); err != nil {
		t.Fatalf("SetOnObject() error = %v", err)
	}
	model.Status = modelsv1alpha1.ModelStatus{
		ObservedGeneration: model.GetGeneration(),
		Phase:              modelsv1alpha1.ModelPhasePublishing,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: modelsv1alpha1.ModelSourceTypeUpload,
		},
	}

	sourceWorkers := &fakeSourceWorkerRuntime{handle: failedSourceWorkerHandle("upload publish failed")}
	uploadSessions := &fakeUploadSessionRuntime{}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, uploadSessions, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if uploadSessions.markFailedCalls != 1 {
		t.Fatalf("expected failed session phase sync, got %d", uploadSessions.markFailedCalls)
	}
	if len(uploadSessions.failedMessages) != 1 || uploadSessions.failedMessages[0] != "upload publish failed" {
		t.Fatalf("unexpected failed messages %#v", uploadSessions.failedMessages)
	}

	var failed modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &failed); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := failed.Status.Phase, modelsv1alpha1.ModelPhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestModelReconcilerRecordsUploadSessionIssuedAuditEvent(t *testing.T) {
	t.Parallel()

	model := testUploadModel()
	auditSink := &fakeAuditSink{}
	uploadSessions := &fakeUploadSessionRuntime{handle: runningUploadSessionHandle()}
	reconciler, _ := newModelReconcilerWithSink(t, &fakeSourceWorkerRuntime{}, uploadSessions, auditSink, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	for _, record := range auditSink.records {
		if record.Reason == auditapp.ReasonUploadSessionIssued {
			return
		}
	}
	t.Fatalf("expected %s event in %#v", auditapp.ReasonUploadSessionIssued, auditSink.records)
}
