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
)

func TestInferModelTypeAndEndpoints(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		task          string
		wantType      modelsv1alpha1.ModelType
		wantEndpoints []string
	}{
		{
			name:          "text generation maps to llm endpoints",
			task:          "text-generation",
			wantType:      modelsv1alpha1.ModelTypeLLM,
			wantEndpoints: []string{string(modelsv1alpha1.ModelEndpointTypeChat), string(modelsv1alpha1.ModelEndpointTypeTextGeneration)},
		},
		{
			name:          "embeddings task maps to embeddings endpoint",
			task:          "embeddings",
			wantType:      modelsv1alpha1.ModelTypeEmbeddings,
			wantEndpoints: []string{string(modelsv1alpha1.ModelEndpointTypeEmbeddings)},
		},
		{
			name:          "translation maps to translation endpoint",
			task:          "translation",
			wantType:      modelsv1alpha1.ModelTypeTranslation,
			wantEndpoints: []string{string(modelsv1alpha1.ModelEndpointTypeTranslation)},
		},
		{
			name:          "unknown task maps to empty type",
			task:          "image-segmentation",
			wantEndpoints: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := inferModelType(tc.task); got != tc.wantType {
				t.Fatalf("inferModelType(%q)=%q, want %q", tc.task, got, tc.wantType)
			}
			gotEndpoints := inferEndpointTypes(tc.task)
			if len(gotEndpoints) != len(tc.wantEndpoints) {
				t.Fatalf("inferEndpointTypes(%q)=%v, want %v", tc.task, gotEndpoints, tc.wantEndpoints)
			}
			for i := range tc.wantEndpoints {
				if gotEndpoints[i] != tc.wantEndpoints[i] {
					t.Fatalf("inferEndpointTypes(%q)=%v, want %v", tc.task, gotEndpoints, tc.wantEndpoints)
				}
			}
		})
	}
}

func TestValidatePreferredRuntime(t *testing.T) {
	t.Parallel()

	t.Run("preferred runtime must belong to allowed set", func(t *testing.T) {
		t.Parallel()
		result := validatePreferredRuntime(&modelsv1alpha1.ModelLaunchPolicy{
			PreferredRuntime: modelsv1alpha1.ModelRuntimeEngineVLLM,
			AllowedRuntimes: []modelsv1alpha1.ModelRuntimeEngine{
				modelsv1alpha1.ModelRuntimeEngineOllama,
			},
		}, []string{"VLLM"})
		if result.Valid || result.Reason != modelsv1alpha1.ModelConditionReasonRuntimeNotSupported {
			t.Fatalf("unexpected result %#v", result)
		}
	})

	t.Run("preferred runtime must be compatible", func(t *testing.T) {
		t.Parallel()
		result := validatePreferredRuntime(&modelsv1alpha1.ModelLaunchPolicy{
			PreferredRuntime: modelsv1alpha1.ModelRuntimeEngineVLLM,
		}, []string{"Ollama"})
		if result.Valid || result.Reason != modelsv1alpha1.ModelConditionReasonRuntimeNotSupported {
			t.Fatalf("unexpected result %#v", result)
		}
	})

	t.Run("compatible preferred runtime is accepted", func(t *testing.T) {
		t.Parallel()
		result := validatePreferredRuntime(&modelsv1alpha1.ModelLaunchPolicy{
			PreferredRuntime: modelsv1alpha1.ModelRuntimeEngineVLLM,
			AllowedRuntimes: []modelsv1alpha1.ModelRuntimeEngine{
				modelsv1alpha1.ModelRuntimeEngineVLLM,
			},
		}, []string{"VLLM"})
		if !result.Valid {
			t.Fatalf("unexpected result %#v", result)
		}
	})

	t.Run("preferred runtime is accepted when compatibility is unresolved", func(t *testing.T) {
		t.Parallel()
		result := validatePreferredRuntime(&modelsv1alpha1.ModelLaunchPolicy{
			PreferredRuntime: modelsv1alpha1.ModelRuntimeEngineVLLM,
		}, nil)
		if !result.Valid {
			t.Fatalf("unexpected result %#v", result)
		}
	})
}

func TestValidatePublishedModelPolicyMismatches(t *testing.T) {
	t.Parallel()

	baseSnapshot := publicationdata.Snapshot{
		Resolved: publicationdata.ResolvedProfile{
			Task:                         "text-generation",
			SupportedEndpointTypes:       []string{"Chat", "TextGeneration"},
			CompatibleRuntimes:           []string{"VLLM"},
			CompatibleAcceleratorVendors: []string{"NVIDIA"},
			CompatiblePrecisions:         []string{"int4"},
		},
	}

	t.Run("model type mismatch fails", func(t *testing.T) {
		t.Parallel()
		result := validatePublishedModel(modelsv1alpha1.ModelSpec{
			ModelType: modelsv1alpha1.ModelTypeEmbeddings,
		}, baseSnapshot)
		if result.Valid || result.Reason != modelsv1alpha1.ModelConditionReasonModelTypeMismatch {
			t.Fatalf("unexpected result %#v", result)
		}
	})

	t.Run("launch policy precision mismatch fails", func(t *testing.T) {
		t.Parallel()
		result := validatePublishedModel(modelsv1alpha1.ModelSpec{
			LaunchPolicy: &modelsv1alpha1.ModelLaunchPolicy{
				AllowedPrecisions: []modelsv1alpha1.ModelPrecision{modelsv1alpha1.ModelPrecisionFP16},
			},
		}, baseSnapshot)
		if result.Valid || result.Reason != modelsv1alpha1.ModelConditionReasonAcceleratorPolicyConflict {
			t.Fatalf("unexpected result %#v", result)
		}
	})

	t.Run("speculative decoding on embeddings fails", func(t *testing.T) {
		t.Parallel()
		result := validatePublishedModel(modelsv1alpha1.ModelSpec{
			Optimization: &modelsv1alpha1.ModelOptimizationPolicy{
				SpeculativeDecoding: &modelsv1alpha1.ModelSpeculativeDecodingPolicy{
					DraftModelRefs: []modelsv1alpha1.ModelReference{
						{Kind: modelsv1alpha1.ModelReferenceKindModel, Name: "draft"},
					},
				},
			},
		}, publicationdata.Snapshot{
			Resolved: publicationdata.ResolvedProfile{Task: "embeddings"},
		})
		if result.Valid || result.Reason != modelsv1alpha1.ModelConditionReasonOptimizationNotSupported {
			t.Fatalf("unexpected result %#v", result)
		}
	})

	t.Run("matching policy succeeds", func(t *testing.T) {
		t.Parallel()
		result := validatePublishedModel(modelsv1alpha1.ModelSpec{
			ModelType: modelsv1alpha1.ModelTypeLLM,
			UsagePolicy: &modelsv1alpha1.ModelUsagePolicy{
				AllowedEndpointTypes: []modelsv1alpha1.ModelEndpointType{
					modelsv1alpha1.ModelEndpointTypeChat,
				},
			},
			LaunchPolicy: &modelsv1alpha1.ModelLaunchPolicy{
				AllowedRuntimes: []modelsv1alpha1.ModelRuntimeEngine{modelsv1alpha1.ModelRuntimeEngineVLLM},
				AllowedAcceleratorVendors: []modelsv1alpha1.ModelAcceleratorVendor{
					modelsv1alpha1.ModelAcceleratorVendorNVIDIA,
				},
				AllowedPrecisions: []modelsv1alpha1.ModelPrecision{modelsv1alpha1.ModelPrecisionINT4},
				PreferredRuntime:  modelsv1alpha1.ModelRuntimeEngineVLLM,
			},
			Optimization: &modelsv1alpha1.ModelOptimizationPolicy{
				SpeculativeDecoding: &modelsv1alpha1.ModelSpeculativeDecodingPolicy{
					DraftModelRefs: []modelsv1alpha1.ModelReference{
						{Kind: modelsv1alpha1.ModelReferenceKindModel, Name: "draft"},
					},
				},
			},
		}, baseSnapshot)
		if !result.Valid || result.Reason != modelsv1alpha1.ModelConditionReasonValidationSucceeded {
			t.Fatalf("unexpected result %#v", result)
		}
	})
}
