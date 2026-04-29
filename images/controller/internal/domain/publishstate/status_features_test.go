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
)

func TestProjectStatusSucceededFiltersUnknownFeatureTypes(t *testing.T) {
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
					Task:                   "image-text-to-text",
					TaskConfidence:         publicationdata.ProfileConfidenceDeclared,
					Format:                 "Safetensors",
					SupportedEndpointTypes: []string{"Chat", "ImageToText"},
					SupportedFeatures:      []string{"VisionInput", "MadeUp"},
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
	want := []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeVisionInput}
	if got := projection.Status.Resolved.SupportedFeatures; !featureTypesEqual(got, want) {
		t.Fatalf("unexpected feature types %#v", got)
	}
}

func TestProjectStatusSucceededProjectsFeatureEvidenceWithoutReliableTask(t *testing.T) {
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
					Task:                   "unknown-task",
					TaskConfidence:         publicationdata.ProfileConfidenceHint,
					Format:                 "Safetensors",
					SupportedEndpointTypes: []string{"Chat"},
					SupportedFeatures:      []string{"ToolCalling", "MadeUp"},
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
	if len(projection.Status.Resolved.SupportedEndpointTypes) != 0 {
		t.Fatalf("hint-only task must not project endpoint types: %#v", projection.Status.Resolved.SupportedEndpointTypes)
	}
	want := []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeToolCalling}
	if got := projection.Status.Resolved.SupportedFeatures; !featureTypesEqual(got, want) {
		t.Fatalf("unexpected feature types %#v", got)
	}
}

func featureTypesEqual(got, want []modelsv1alpha1.ModelFeatureType) bool {
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
