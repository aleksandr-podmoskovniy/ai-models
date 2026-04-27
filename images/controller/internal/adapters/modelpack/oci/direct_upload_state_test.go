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

func TestDirectUploadCheckpointRejectsStaleCompletedLayerWithSameKey(t *testing.T) {
	t.Parallel()

	plan := publishLayerDescriptor{
		Digest:      "sha256:new",
		DiffID:      "sha256:new",
		Size:        100,
		MediaType:   "application/vnd.cncf.model.model.v1.raw",
		TargetPath:  "model/weights.gguf",
		Base:        modelpackports.LayerBaseModel,
		Format:      modelpackports.LayerFormatRaw,
		Compression: modelpackports.LayerCompressionNone,
	}
	checkpoint := &directUploadCheckpoint{
		state: modelpackports.DirectUploadState{
			CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{
				{
					Key:         directUploadLayerKey(plan),
					Digest:      "sha256:old",
					DiffID:      "sha256:old",
					SizeBytes:   100,
					MediaType:   plan.MediaType,
					TargetPath:  plan.TargetPath,
					Base:        plan.Base,
					Format:      plan.Format,
					Compression: plan.Compression,
				},
			},
		},
	}

	if _, found, err := checkpoint.completedLayer(plan, 100); err == nil || found {
		t.Fatalf("completedLayer() = found %v, error %v; want digest mismatch rejection", found, err)
	}
}

func TestDirectUploadCheckpointRejectsCompletedLayerSizeMismatch(t *testing.T) {
	t.Parallel()

	plan := publishLayerDescriptor{
		Digest:      "sha256:new",
		DiffID:      "sha256:new",
		Size:        100,
		MediaType:   "application/vnd.cncf.model.model.v1.raw",
		TargetPath:  "model/weights.gguf",
		Base:        modelpackports.LayerBaseModel,
		Format:      modelpackports.LayerFormatRaw,
		Compression: modelpackports.LayerCompressionNone,
	}
	layer := stateLayerFromDescriptor(plan)
	layer.SizeBytes = 99
	checkpoint := &directUploadCheckpoint{
		state: modelpackports.DirectUploadState{
			CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{layer},
		},
	}

	if _, found, err := checkpoint.completedLayer(plan, 100); err == nil || found {
		t.Fatalf("completedLayer() = found %v, error %v; want size mismatch rejection", found, err)
	}
}

func TestDirectUploadCheckpointRejectsCompletedLayerShapeMismatch(t *testing.T) {
	t.Parallel()

	plan := publishLayerDescriptor{
		MediaType:   "application/vnd.cncf.model.model.v1.raw",
		TargetPath:  "model/weights.gguf",
		Base:        modelpackports.LayerBaseModel,
		Format:      modelpackports.LayerFormatRaw,
		Compression: modelpackports.LayerCompressionNone,
	}
	tests := []struct {
		name  string
		layer modelpackports.DirectUploadLayerDescriptor
	}{
		{
			name: "base",
			layer: modelpackports.DirectUploadLayerDescriptor{
				Key:         directUploadLayerKey(plan),
				SizeBytes:   100,
				MediaType:   plan.MediaType,
				TargetPath:  plan.TargetPath,
				Base:        modelpackports.LayerBaseModelConfig,
				Format:      plan.Format,
				Compression: plan.Compression,
			},
		},
		{
			name: "format",
			layer: modelpackports.DirectUploadLayerDescriptor{
				Key:         directUploadLayerKey(plan),
				SizeBytes:   100,
				MediaType:   plan.MediaType,
				TargetPath:  plan.TargetPath,
				Base:        plan.Base,
				Format:      modelpackports.LayerFormatTar,
				Compression: plan.Compression,
			},
		},
		{
			name: "compression",
			layer: modelpackports.DirectUploadLayerDescriptor{
				Key:         directUploadLayerKey(plan),
				SizeBytes:   100,
				MediaType:   plan.MediaType,
				TargetPath:  plan.TargetPath,
				Base:        plan.Base,
				Format:      plan.Format,
				Compression: modelpackports.LayerCompressionGzip,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			checkpoint := &directUploadCheckpoint{
				state: modelpackports.DirectUploadState{
					CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{test.layer},
				},
			}
			if _, found, err := checkpoint.completedLayer(plan, 100); err == nil || found {
				t.Fatalf("completedLayer() = found %v, error %v; want stale checkpoint rejection", found, err)
			}
		})
	}
}

func TestDirectUploadCheckpointReusesMatchingCompletedLayer(t *testing.T) {
	t.Parallel()

	descriptor := publishLayerDescriptor{
		Digest:      "sha256:new",
		DiffID:      "sha256:new",
		Size:        100,
		MediaType:   "application/vnd.cncf.model.model.v1.raw",
		TargetPath:  "model/weights.gguf",
		Base:        modelpackports.LayerBaseModel,
		Format:      modelpackports.LayerFormatRaw,
		Compression: modelpackports.LayerCompressionNone,
	}
	checkpoint := &directUploadCheckpoint{
		state: modelpackports.DirectUploadState{
			CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{stateLayerFromDescriptor(descriptor)},
		},
	}

	completed, found, err := checkpoint.completedLayer(descriptor, descriptor.Size)
	if err != nil {
		t.Fatalf("completedLayer() error = %v", err)
	}
	if !found {
		t.Fatal("completedLayer() found = false, want true")
	}
	if completed != descriptor {
		t.Fatalf("completedLayer() = %#v, want %#v", completed, descriptor)
	}
}
