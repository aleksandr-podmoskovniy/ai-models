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

package oci

import (
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestValidatePayloadAcceptsCompressedAndAuxiliaryLayers(t *testing.T) {
	t.Parallel()

	weightConfigType, err := buildLayerMediaType(modelpackports.LayerBaseModelConfig, modelpackports.LayerFormatRaw, modelpackports.LayerCompressionNone)
	if err != nil {
		t.Fatalf("buildLayerMediaType(weight.config raw) error = %v", err)
	}
	codeType, err := buildLayerMediaType(modelpackports.LayerBaseCode, modelpackports.LayerFormatTar, modelpackports.LayerCompressionGzip)
	if err != nil {
		t.Fatalf("buildLayerMediaType(code tar+gzip) error = %v", err)
	}
	datasetType, err := buildLayerMediaType(modelpackports.LayerBaseDataset, modelpackports.LayerFormatTar, modelpackports.LayerCompressionZstd)
	if err != nil {
		t.Fatalf("buildLayerMediaType(dataset tar+zstd) error = %v", err)
	}
	docType, err := buildLayerMediaType(modelpackports.LayerBaseDoc, modelpackports.LayerFormatRaw, modelpackports.LayerCompressionNone)
	if err != nil {
		t.Fatalf("buildLayerMediaType(doc raw) error = %v", err)
	}

	err = ValidatePayload(InspectPayload{
		"digest": "sha256:deadbeef",
		"manifest": map[string]any{
			"schemaVersion": 2,
			"artifactType":  ModelPackArtifactType,
			"config": map[string]any{
				"mediaType": ModelPackConfigMediaType,
				"digest":    "sha256:config",
				"size":      10,
			},
			"layers": []any{
				map[string]any{
					"mediaType": ModelPackWeightLayerType,
					"digest":    "sha256:model",
					"size":      11,
					"annotations": map[string]any{
						ModelPackFilepathAnnotation: "model",
					},
				},
				map[string]any{
					"mediaType": weightConfigType,
					"digest":    "sha256:modelconfig",
					"size":      12,
					"annotations": map[string]any{
						ModelPackFilepathAnnotation: "model/config.json",
					},
				},
				map[string]any{
					"mediaType": codeType,
					"digest":    "sha256:code",
					"size":      13,
					"annotations": map[string]any{
						ModelPackFilepathAnnotation: "code",
					},
				},
				map[string]any{
					"mediaType": datasetType,
					"digest":    "sha256:data",
					"size":      14,
					"annotations": map[string]any{
						ModelPackFilepathAnnotation: "datasets",
					},
				},
				map[string]any{
					"mediaType": docType,
					"digest":    "sha256:doc",
					"size":      15,
					"annotations": map[string]any{
						ModelPackFilepathAnnotation: "docs/README.md",
					},
				},
			},
		},
		"configBlob": map[string]any{
			"descriptor": map[string]any{"name": "model"},
			"modelfs": map[string]any{
				"type":    "layers",
				"diffIds": []any{"sha256:1", "sha256:2", "sha256:3", "sha256:4", "sha256:5"},
			},
			"config": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("ValidatePayload() error = %v", err)
	}
}
