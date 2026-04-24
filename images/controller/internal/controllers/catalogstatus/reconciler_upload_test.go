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
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	if got, want := updated.Status.Progress, "17%"; got != want {
		t.Fatalf("unexpected progress %q", got)
	}
}

func TestModelReconcilerKeepsUploadSessionAndMarksPublishingOnRawStageHandoff(t *testing.T) {
	t.Parallel()

	model := testUploadModel()
	deleted := false
	uploadSessions := &fakeUploadSessionRuntime{handle: succeededUploadSessionHandle(t, &deleted)}
	sourceWorkers := &fakeSourceWorkerRuntime{handle: runningSourceWorkerHandle()}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, uploadSessions, model)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.Requeue {
		t.Fatalf("expected staged upload to requeue, got %#v", result)
	}
	if deleted {
		t.Fatal("upload session runtime must stay alive until ready status is projected")
	}
	if uploadSessions.markPublishingCalls != 1 {
		t.Fatalf("expected upload session publishing callback after staging handoff, got %d calls", uploadSessions.markPublishingCalls)
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := updated.Status.Phase, modelsv1alpha1.ModelPhasePublishing; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestModelReconcilerMarksCompletedUploadSessionAfterReadyProjection(t *testing.T) {
	t.Parallel()

	model := testUploadModel()
	uploadSessions := &fakeUploadSessionRuntime{handle: succeededUploadSessionHandle(t, nil)}
	sourceWorkers := &fakeSourceWorkerRuntime{handles: []*publicationports.SourceWorkerHandle{
		runningSourceWorkerHandle(),
		succeededSourceWorkerHandle(t, nil),
		succeededSourceWorkerHandle(t, nil),
	}}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, uploadSessions, model)

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
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("third Reconcile() error = %v", err)
	}
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("fourth Reconcile() error = %v", err)
	}

	if uploadSessions.markCompletedCalls != 1 {
		t.Fatalf("expected upload session completion callback, got %d calls", uploadSessions.markCompletedCalls)
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
	uploadSessions := &fakeUploadSessionRuntime{handle: succeededUploadSessionHandle(t, nil)}
	sourceWorkers := &fakeSourceWorkerRuntime{handles: []*publicationports.SourceWorkerHandle{
		runningSourceWorkerHandle(),
		failedSourceWorkerHandle("publish failed"),
	}}
	reconciler, kubeClient := newModelReconciler(t, sourceWorkers, uploadSessions, model)

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
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("third Reconcile() error = %v", err)
	}

	if uploadSessions.markFailedCalls != 1 {
		t.Fatalf("expected upload session failure callback, got %d calls", uploadSessions.markFailedCalls)
	}
	if len(uploadSessions.failedMessages) != 1 || uploadSessions.failedMessages[0] != "publish failed" {
		t.Fatalf("unexpected upload session failure messages %#v", uploadSessions.failedMessages)
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
	reconciler, _ := newModelReconcilerWithSink(t, &fakeSourceWorkerRuntime{}, &fakeUploadSessionRuntime{handle: runningUploadSessionHandle()}, auditSink, model)

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
