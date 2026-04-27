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
		modelsv1alpha1.ModelStatus{Progress: "98%"},
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
					Task:                          "text-generation",
					TaskConfidence:                publicationdata.ProfileConfidenceDerived,
					Family:                        "deepseek",
					FamilyConfidence:              publicationdata.ProfileConfidenceExact,
					Architecture:                  "DeepseekForCausalLM",
					ArchitectureConfidence:        publicationdata.ProfileConfidenceExact,
					Format:                        "Safetensors",
					ParameterCount:                8_000_000_000,
					ParameterCountConfidence:      publicationdata.ProfileConfidenceExact,
					Quantization:                  "bnb-nf4",
					QuantizationConfidence:        publicationdata.ProfileConfidenceExact,
					ContextWindowTokens:           8192,
					ContextWindowTokensConfidence: publicationdata.ProfileConfidenceExact,
					SupportedEndpointTypes:        []string{"Chat", "TextGeneration"},
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
	if projection.Status.Progress != "" {
		t.Fatalf("ready status must clear running progress, got %q", projection.Status.Progress)
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
	ready := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionReady))
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("expected ready condition, got %#v", ready)
	}
	metadata := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionMetadataResolved))
	if metadata == nil || metadata.Status != metav1.ConditionTrue || metadata.Reason != string(modelsv1alpha1.ModelConditionReasonModelMetadataCalculated) {
		t.Fatalf("unexpected metadata condition %#v", metadata)
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
					TaskConfidence:         publicationdata.ProfileConfidenceDerived,
					Format:                 "Safetensors",
					SupportedEndpointTypes: []string{"Chat", "TextGeneration"},
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
	metadata := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionMetadataResolved))
	if metadata == nil || metadata.Status != metav1.ConditionTrue {
		t.Fatalf("unexpected metadata condition %#v", metadata)
	}
}

func TestProjectStatusSucceededOmitsHintOnlyProfileFields(t *testing.T) {
	t.Parallel()

	projection, err := ProjectStatus(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelSpec{},
		5,
		modelsv1alpha1.ModelSourceTypeUpload,
		Observation{
			Phase: OperationPhaseSucceeded,
			Snapshot: &publicationdata.Snapshot{
				Source: publicationdata.SourceProvenance{Type: modelsv1alpha1.ModelSourceTypeUpload},
				Artifact: publicationdata.PublishedArtifact{
					Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
					URI:  "registry.example/model@sha256:deadbeef",
				},
				Resolved: publicationdata.ResolvedProfile{
					Task:                     "text-generation",
					TaskConfidence:           publicationdata.ProfileConfidenceExact,
					Family:                   "deepseek-r1",
					FamilyConfidence:         publicationdata.ProfileConfidenceHint,
					Format:                   "GGUF",
					ParameterCount:           8_000_000_000,
					ParameterCountConfidence: publicationdata.ProfileConfidenceHint,
					Quantization:             "q4_k_m",
					QuantizationConfidence:   publicationdata.ProfileConfidenceHint,
					SupportedEndpointTypes:   []string{"Chat", "TextGeneration"},
				},
			},
			CleanupHandle: &cleanuphandle.Handle{
				Kind: cleanuphandle.KindBackendArtifact,
				Backend: &cleanuphandle.BackendArtifactHandle{
					Reference: "registry.example/model@sha256:deadbeef",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Resolved.Format, modelsv1alpha1.ModelInputFormatGGUF; got != want {
		t.Fatalf("unexpected format %q", got)
	}
	if projection.Status.Resolved.Family != "" {
		t.Fatalf("hint-only family must not be public, got %q", projection.Status.Resolved.Family)
	}
	if projection.Status.Resolved.ParameterCount != nil {
		t.Fatalf("hint-only parameter count must not be public, got %#v", projection.Status.Resolved.ParameterCount)
	}
	if projection.Status.Resolved.Quantization != nil {
		t.Fatalf("hint-only quantization must not be public, got %#v", projection.Status.Resolved.Quantization)
	}
	metadata := apimeta.FindStatusCondition(projection.Status.Conditions, string(modelsv1alpha1.ModelConditionMetadataResolved))
	if metadata == nil || metadata.Reason != string(modelsv1alpha1.ModelConditionReasonModelMetadataPartial) {
		t.Fatalf("unexpected metadata condition %#v", metadata)
	}
}

func TestProjectStatusSucceededFiltersUnknownEndpointTypes(t *testing.T) {
	t.Parallel()

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
					Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
					URI:  "registry.example/model@sha256:deadbeef",
				},
				Resolved: publicationdata.ResolvedProfile{
					Task:                   "text-generation",
					TaskConfidence:         publicationdata.ProfileConfidenceExact,
					Format:                 "Safetensors",
					SupportedEndpointTypes: []string{"Chat", "MadeUp"},
				},
			},
			CleanupHandle: &cleanuphandle.Handle{
				Kind: cleanuphandle.KindBackendArtifact,
				Backend: &cleanuphandle.BackendArtifactHandle{
					Reference: "registry.example/model@sha256:deadbeef",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if got, want := projection.Status.Resolved.SupportedEndpointTypes, []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeChat}; !endpointTypesEqual(got, want) {
		t.Fatalf("unexpected endpoint types %#v", got)
	}
}

func TestProjectStatusSucceededFiltersUnknownFormat(t *testing.T) {
	t.Parallel()

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
					Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
					URI:  "registry.example/model@sha256:deadbeef",
				},
				Resolved: publicationdata.ResolvedProfile{
					Task:                   "text-generation",
					TaskConfidence:         publicationdata.ProfileConfidenceExact,
					Format:                 "ONNX",
					SupportedEndpointTypes: []string{"Chat"},
				},
			},
			CleanupHandle: &cleanuphandle.Handle{
				Kind: cleanuphandle.KindBackendArtifact,
				Backend: &cleanuphandle.BackendArtifactHandle{
					Reference: "registry.example/model@sha256:deadbeef",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("ProjectStatus() error = %v", err)
	}
	if projection.Status.Resolved.Format != "" {
		t.Fatalf("unknown format must not be public, got %q", projection.Status.Resolved.Format)
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

func endpointTypesEqual(got, want []modelsv1alpha1.ModelEndpointType) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
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
