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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		modelsv1alpha1.ModelSpec{},
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
					Architecture:                 "DeepseekForCausalLM",
					Format:                       "Safetensors",
					ParameterCount:               8_000_000_000,
					Quantization:                 "bnb-nf4",
					ContextWindowTokens:          8192,
					SupportedEndpointTypes:       []string{"Chat", "TextGeneration"},
					CompatibleRuntimes:           []string{"VLLM"},
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
	if projection.Status.Resolved == nil {
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
	validated := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionValidated))
	if validated == nil || validated.Status != metav1.ConditionTrue || validated.Reason != string(modelsv1alpha1.ModelConditionReasonValidationSucceeded) {
		t.Fatalf("unexpected validated condition %#v", validated)
	}
}

func TestProjectStatusSucceededAcceptsCalculatedMetadataWithoutSpecPolicy(t *testing.T) {
	t.Parallel()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: "registry.example/model@sha256:deadbeef",
		},
	}

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
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
					SupportedEndpointTypes: []string{"Chat", "TextGeneration"},
					CompatibleRuntimes:     []string{"VLLM"},
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
	validated := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionValidated))
	if validated == nil || validated.Status != metav1.ConditionTrue || validated.Reason != string(modelsv1alpha1.ModelConditionReasonValidationSucceeded) {
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
