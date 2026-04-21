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

func TestPublishedModelPathPrefersWeightRootOverLeadingConfigLayer(t *testing.T) {
	t.Parallel()

	got := publishedModelPath([]publishLayerDescriptor{
		{
			TargetPath: "model/config.json",
			Base:       modelpackports.LayerBaseModelConfig,
			Format:     modelpackports.LayerFormatRaw,
		},
		{
			TargetPath: "model",
			Base:       modelpackports.LayerBaseModel,
			Format:     modelpackports.LayerFormatTar,
		},
	})

	if got != "model" {
		t.Fatalf("publishedModelPath() = %q, want model", got)
	}
}

func TestPublishedModelPathCollapsesSplitWeightFilesToCommonRoot(t *testing.T) {
	t.Parallel()

	got := publishedModelPath([]publishLayerDescriptor{
		{
			TargetPath: "model/model-00001-of-00002.safetensors",
			Base:       modelpackports.LayerBaseModel,
			Format:     modelpackports.LayerFormatRaw,
		},
		{
			TargetPath: "model/model-00002-of-00002.safetensors",
			Base:       modelpackports.LayerBaseModel,
			Format:     modelpackports.LayerFormatRaw,
		},
	})

	if got != "model" {
		t.Fatalf("publishedModelPath() = %q, want model", got)
	}
}

func TestPublishedModelPathKeepsSingleRawWeightFile(t *testing.T) {
	t.Parallel()

	got := publishedModelPath([]publishLayerDescriptor{
		{
			TargetPath: "model.gguf",
			Base:       modelpackports.LayerBaseModel,
			Format:     modelpackports.LayerFormatRaw,
		},
	})

	if got != "model.gguf" {
		t.Fatalf("publishedModelPath() = %q, want model.gguf", got)
	}
}
