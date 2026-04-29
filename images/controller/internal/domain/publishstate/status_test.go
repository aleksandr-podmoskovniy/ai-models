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

package publishstate

import (
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInitialStatusUploadStartsPending(t *testing.T) {
	t.Parallel()

	status := InitialStatus(modelsv1alpha1.ModelStatus{}, 5, modelsv1alpha1.ModelSourceTypeUpload)
	if got, want := status.Phase, modelsv1alpha1.ModelPhasePending; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if got, want := status.Progress, "0%"; got != want {
		t.Fatalf("unexpected progress %q", got)
	}
}

func TestProjectStatusRunningNonUploadStaysPublishing(t *testing.T) {
	t.Parallel()

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		Observation{Phase: OperationPhaseRunning},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhasePublishing; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestProjectStatusRunningUsesObservationMessage(t *testing.T) {
	t.Parallel()

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		Observation{
			Phase:           OperationPhaseRunning,
			ConditionReason: modelsv1alpha1.ModelConditionReasonPublishing,
			Message:         "123/456 bytes uploaded into the internal registry",
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}

	artifactResolved := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionArtifactResolved))
	if artifactResolved == nil || artifactResolved.Message != "123/456 bytes uploaded into the internal registry" || artifactResolved.Reason != string(modelsv1alpha1.ModelConditionReasonPublishing) {
		t.Fatalf("unexpected artifact resolved condition %#v", artifactResolved)
	}
	ready := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionReady))
	if ready == nil || ready.Message != "123/456 bytes uploaded into the internal registry" || ready.Reason != string(modelsv1alpha1.ModelConditionReasonPublishing) {
		t.Fatalf("unexpected ready condition %#v", ready)
	}
}

func TestProjectStatusRunningNonUploadProjectsPublicProgress(t *testing.T) {
	t.Parallel()

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		Observation{
			Phase:           OperationPhaseRunning,
			ConditionReason: modelsv1alpha1.ModelConditionReasonPublishing,
			Progress:        "37%",
			Message:         "384/1024 bytes uploaded into the internal registry",
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Progress, "37%"; got != want {
		t.Fatalf("unexpected progress %q", got)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhasePublishing; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestProjectStatusRunningUploadWithoutSessionShowsPublishing(t *testing.T) {
	t.Parallel()

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeUpload,
		Observation{Phase: OperationPhaseRunning},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhasePublishing; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if !projection.Requeue {
		t.Fatal("expected running upload without session to requeue")
	}
}

func TestProjectStatusRunningUploadWithSession(t *testing.T) {
	t.Parallel()

	expiresAt := metav1.NewTime(time.Unix(1712345678, 0).UTC())
	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeUpload,
		Observation{
			Phase:    OperationPhaseRunning,
			Progress: "17%",
			Upload: &modelsv1alpha1.ModelUploadStatus{
				ExpiresAt:  &expiresAt,
				Repository: "registry.example/upload",
				External:   "https://ai-models.example.com/upload/token",
				InCluster:  "http://upload-a.d8-ai-models.svc:8444/upload/token",
			},
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhaseWaitForUpload; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if projection.Status.Upload == nil || projection.Status.Upload.InCluster != "http://upload-a.d8-ai-models.svc:8444/upload/token" {
		t.Fatalf("unexpected upload status %#v", projection.Status.Upload)
	}
	if got, want := projection.Status.Progress, "17%"; got != want {
		t.Fatalf("unexpected progress %q", got)
	}
	artifactResolved := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionArtifactResolved))
	if artifactResolved == nil || artifactResolved.Reason != string(modelsv1alpha1.ModelConditionReasonWaitingForUserUpload) {
		t.Fatalf("expected waiting upload artifact condition, got %#v", artifactResolved)
	}
}

func TestProjectStatusStagedRequeuesIntoPublishPhase(t *testing.T) {
	t.Parallel()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:   "ai-models",
			Key:      "raw/1111-2222/model.gguf",
			FileName: "model.gguf",
		},
	}

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeUpload,
		Observation{
			Phase:         OperationPhaseStaged,
			CleanupHandle: &handle,
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhasePublishing; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if !projection.Requeue {
		t.Fatal("expected staged upload to requeue into publish worker")
	}
	if projection.CleanupHandle == nil || projection.CleanupHandle.Kind != cleanuphandle.KindUploadStaging {
		t.Fatalf("unexpected cleanup handle %#v", projection.CleanupHandle)
	}
}

func TestProjectStatusStagedRequiresCleanupHandle(t *testing.T) {
	t.Parallel()

	_, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeUpload,
		Observation{Phase: OperationPhaseStaged},
	)
	if err == nil {
		t.Fatal("expected missing staged cleanup handle error")
	}
}

func TestProjectStatusFailed(t *testing.T) {
	t.Parallel()

	current := modelsv1alpha1.ModelStatus{
		Conditions: []metav1.Condition{{
			Type:   "DeckhouseSpecific",
			Status: metav1.ConditionTrue,
		}},
	}

	projection, err := ProjectStatus(
		current,
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		Observation{
			Phase:   OperationPhaseFailed,
			Message: "download failed",
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if apimeta.FindStatusCondition(projection.Status.Conditions, "DeckhouseSpecific") == nil {
		t.Fatal("expected non-publication condition to be preserved")
	}
}

func TestProjectStatusUnsupportedSourceFailureOmitsResolvedType(t *testing.T) {
	t.Parallel()

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		"",
		Observation{
			Phase:           OperationPhaseFailed,
			ConditionReason: modelsv1alpha1.ModelConditionReasonUnsupportedSource,
			Message:         `unsupported source URL host "downloads.example.com"`,
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if projection.Status.Source != nil {
		t.Fatalf("unexpected source status %#v", projection.Status.Source)
	}
	artifactResolved := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionArtifactResolved))
	if artifactResolved == nil || artifactResolved.Reason != string(modelsv1alpha1.ModelConditionReasonUnsupportedSource) {
		t.Fatalf("unexpected artifact resolved condition %#v", artifactResolved)
	}
	ready := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionReady))
	if ready == nil || ready.Reason != string(modelsv1alpha1.ModelConditionReasonFailed) {
		t.Fatalf("unexpected ready condition %#v", ready)
	}
}
