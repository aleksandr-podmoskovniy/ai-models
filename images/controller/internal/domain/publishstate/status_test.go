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
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAcceptedStatusUploadStartsPending(t *testing.T) {
	t.Parallel()

	status := AcceptedStatus(modelsv1alpha1.ModelStatus{}, 5, modelsv1alpha1.ModelSourceTypeUpload)
	if got, want := status.Phase, modelsv1alpha1.ModelPhasePending; got != want {
		t.Fatalf("unexpected phase %q", got)
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
			Phase: OperationPhaseRunning,
			Upload: &modelsv1alpha1.ModelUploadStatus{
				ExpiresAt:    &expiresAt,
				Repository:   "registry.example/upload",
				ExternalURL:  "https://ai-models.example.com/upload/token",
				InClusterURL: "http://upload-a.d8-ai-models.svc:8444/upload/token",
			},
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhaseWaitForUpload; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if projection.Status.Upload == nil || projection.Status.Upload.InClusterURL != "http://upload-a.d8-ai-models.svc:8444/upload/token" {
		t.Fatalf("unexpected upload status %#v", projection.Status.Upload)
	}
	uploadReady := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionUploadReady))
	if uploadReady == nil || uploadReady.Status != metav1.ConditionTrue {
		t.Fatalf("expected upload-ready condition, got %#v", uploadReady)
	}
}

func TestProjectStatusStagedRequeuesIntoPublishPhase(t *testing.T) {
	t.Parallel()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:   "ai-models",
			Key:      "uploaded-model-staging/1111-2222/model.gguf",
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
		modelsv1alpha1.ModelSourceTypeHTTP,
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

func TestProjectStatusSucceeded(t *testing.T) {
	t.Parallel()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Artifact: &cleanuphandle.ArtifactSnapshot{
			Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:    "registry.example/model@sha256:deadbeef",
			Digest: "sha256:deadbeef",
		},
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: "registry.example/model@sha256:deadbeef",
		},
	}
	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{
			ModelType: modelsv1alpha1.ModelTypeLLM,
			UsagePolicy: &modelsv1alpha1.ModelUsagePolicy{
				AllowedEndpointTypes: []modelsv1alpha1.ModelEndpointType{
					modelsv1alpha1.ModelEndpointTypeChat,
				},
			},
			LaunchPolicy: &modelsv1alpha1.ModelLaunchPolicy{
				AllowedRuntimes: []modelsv1alpha1.ModelRuntimeEngine{
					modelsv1alpha1.ModelRuntimeEngineKServe,
				},
				PreferredRuntime: modelsv1alpha1.ModelRuntimeEngineKServe,
				AllowedAcceleratorVendors: []modelsv1alpha1.ModelAcceleratorVendor{
					modelsv1alpha1.ModelAcceleratorVendorNVIDIA,
				},
				AllowedPrecisions: []modelsv1alpha1.ModelPrecision{
					modelsv1alpha1.ModelPrecisionINT4,
				},
			},
			Optimization: &modelsv1alpha1.ModelOptimizationPolicy{
				SpeculativeDecoding: &modelsv1alpha1.ModelSpeculativeDecodingPolicy{
					DraftModelRefs: []modelsv1alpha1.ModelReference{
						{Kind: modelsv1alpha1.ModelReferenceKindClusterModel, Name: "deepseek-r1-draft"},
					},
				},
			},
		},
		5,
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		Observation{
			Phase: OperationPhaseSucceeded,
			Snapshot: &publicationdata.Snapshot{
				Source: publicationdata.SourceProvenance{
					Type:             modelsv1alpha1.ModelSourceTypeHuggingFace,
					ResolvedRevision: "abc123",
				},
				Artifact: publicationdata.PublishedArtifact{
					Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
					URI:       "registry.example/model@sha256:deadbeef",
					Digest:    "sha256:deadbeef",
					MediaType: "application/vnd.cncf.model.manifest.v1+json",
					SizeBytes: 42,
				},
				Resolved: publicationdata.ResolvedProfile{
					Task:                         "text-generation",
					Framework:                    "transformers",
					Family:                       "deepseek",
					License:                      "apache-2.0",
					Architecture:                 "DeepseekForCausalLM",
					Format:                       "Safetensors",
					ParameterCount:               8_000_000_000,
					Quantization:                 "bnb-nf4",
					ContextWindowTokens:          8192,
					SourceRepoID:                 "deepseek-ai/DeepSeek-R1",
					SupportedEndpointTypes:       []string{"OpenAIChatCompletions", "OpenAICompletions"},
					CompatibleRuntimes:           []string{"KServe"},
					CompatibleAcceleratorVendors: []string{"NVIDIA", "AMD"},
					CompatiblePrecisions:         []string{"int4"},
					MinimumLaunch: publicationdata.MinimumLaunch{
						PlacementType:        "GPU",
						AcceleratorCount:     1,
						AcceleratorMemoryGiB: 24,
						SharingMode:          "Exclusive",
					},
				},
			},
			CleanupHandle: &handle,
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhaseReady; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if projection.CleanupHandle == nil || projection.CleanupHandle.Backend == nil {
		t.Fatalf("expected cleanup handle in projection, got %#v", projection.CleanupHandle)
	}
	if projection.Status.Artifact == nil || projection.Status.Artifact.URI != "registry.example/model@sha256:deadbeef" {
		t.Fatalf("unexpected artifact status %#v", projection.Status.Artifact)
	}
	if projection.Status.Resolved == nil || projection.Status.Resolved.SourceRepoID != "deepseek-ai/DeepSeek-R1" {
		t.Fatalf("unexpected resolved status %#v", projection.Status.Resolved)
	}
	if projection.Status.Resolved.ParameterCount == nil || *projection.Status.Resolved.ParameterCount != 8_000_000_000 {
		t.Fatalf("unexpected parameter count %#v", projection.Status.Resolved.ParameterCount)
	}
	if projection.Status.Resolved.Quantization == nil || *projection.Status.Resolved.Quantization != "bnb-nf4" {
		t.Fatalf("unexpected quantization %#v", projection.Status.Resolved.Quantization)
	}
	if projection.Status.Resolved.MinimumLaunch == nil || projection.Status.Resolved.MinimumLaunch.PlacementType != "GPU" {
		t.Fatalf("unexpected minimum launch %#v", projection.Status.Resolved.MinimumLaunch)
	}
	ready := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionReady))
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("expected ready condition, got %#v", ready)
	}
}

func TestProjectStatusSucceededValidationFailure(t *testing.T) {
	t.Parallel()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: "registry.example/model@sha256:deadbeef",
		},
	}

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{
			ModelType: modelsv1alpha1.ModelTypeEmbeddings,
		},
		5,
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		Observation{
			Phase: OperationPhaseSucceeded,
			Snapshot: &publicationdata.Snapshot{
				Source: publicationdata.SourceProvenance{Type: modelsv1alpha1.ModelSourceTypeHuggingFace},
				Artifact: publicationdata.PublishedArtifact{
					Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
					URI:       "registry.example/model@sha256:deadbeef",
					Digest:    "sha256:deadbeef",
					MediaType: "application/vnd.cncf.model.manifest.v1+json",
				},
				Resolved: publicationdata.ResolvedProfile{
					Task:                   "text-generation",
					Framework:              "transformers",
					Format:                 "Safetensors",
					SupportedEndpointTypes: []string{"OpenAIChatCompletions", "OpenAICompletions"},
					CompatibleRuntimes:     []string{"KServe"},
				},
			},
			CleanupHandle: &handle,
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Phase, modelsv1alpha1.ModelPhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	validated := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionValidated))
	if validated == nil || validated.Status != metav1.ConditionFalse || validated.Reason != string(modelsv1alpha1.ModelConditionReasonModelTypeMismatch) {
		t.Fatalf("unexpected validated condition %#v", validated)
	}
}

func TestProjectStatusSucceededRequiresSnapshot(t *testing.T) {
	t.Parallel()

	_, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		Observation{Phase: OperationPhaseSucceeded},
	)
	if err == nil {
		t.Fatal("expected missing snapshot error")
	}
}

func TestProjectStatusSucceededRequiresCleanupHandle(t *testing.T) {
	t.Parallel()

	_, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		Observation{
			Phase:    OperationPhaseSucceeded,
			Snapshot: &publicationdata.Snapshot{},
		},
	)
	if err == nil {
		t.Fatal("expected missing cleanup handle error")
	}
}
