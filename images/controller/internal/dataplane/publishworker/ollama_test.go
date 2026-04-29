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

package publishworker

import (
	"context"
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestRunPublishesOllamaRemoteObjectSource(t *testing.T) {
	t.Parallel()

	previous := fetchRemoteModelFunc
	fetchRemoteModelFunc = func(_ context.Context, options sourcefetch.RemoteOptions) (sourcefetch.RemoteResult, error) {
		if got, want := options.URL, "https://ollama.com/library/qwen3.6:latest"; got != want {
			t.Fatalf("unexpected source URL %q", got)
		}
		return sourcefetch.RemoteResult{
			SourceType:    modelsv1alpha1.ModelSourceTypeOllama,
			InputFormat:   modelsv1alpha1.ModelInputFormatGGUF,
			SelectedFiles: []string{"qwen3.6-latest-q4_k_m.gguf"},
			ObjectSource: &sourcefetch.RemoteObjectSource{
				Reader: fakeRemoteObjectReader{},
				Files: []sourcefetch.RemoteObjectFile{{
					SourcePath: "https://registry.ollama.ai/v2/library/qwen3.6/blobs/sha256:feed",
					TargetPath: "qwen3.6-latest-q4_k_m.gguf",
					SizeBytes:  42,
				}},
			},
			ProfileSummary: &sourcefetch.RemoteProfileSummary{
				ModelFileName:  "qwen3.6-latest-q4_k_m.gguf",
				ModelSizeBytes: 42,
				Family:         "qwen35moe",
				Quantization:   "Q4_K_M",
			},
			Provenance: sourcefetch.RemoteProvenance{
				ExternalReference: "ollama.com/library/qwen3.6:latest",
				ResolvedRevision:  "latest@sha256:feed",
			},
		}, nil
	}
	t.Cleanup(func() { fetchRemoteModelFunc = previous })

	result, err := Run(context.Background(), Options{
		SourceType:              modelsv1alpha1.ModelSourceTypeOllama,
		ArtifactURI:             "registry.example.com/ai-models/catalog/model",
		SourceURL:               "https://ollama.com/library/qwen3.6:latest",
		OCIDirectUploadEndpoint: "https://dmcr.example.com/direct",
		ModelPackPublisher: fakePublisher{
			onPublish: func(input modelpackports.PublishInput) error {
				if len(input.Layers) != 1 || input.Layers[0].ObjectSource == nil {
					t.Fatalf("expected one object-source layer, got %#v", input.Layers)
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := result.Source.Type, modelsv1alpha1.ModelSourceTypeOllama; got != want {
		t.Fatalf("unexpected source type %q", got)
	}
	if got, want := result.Resolved.Family, "qwen35moe"; got != want {
		t.Fatalf("unexpected resolved family %q", got)
	}
	if got, want := result.Resolved.Quantization, "Q4_K_M"; got != want {
		t.Fatalf("unexpected resolved quantization %q", got)
	}
}

func TestRunRejectsOllamaWithoutSourceURL(t *testing.T) {
	t.Parallel()

	_, err := Run(context.Background(), Options{
		SourceType:              modelsv1alpha1.ModelSourceTypeOllama,
		ArtifactURI:             "registry.example.com/ai-models/catalog/model",
		OCIDirectUploadEndpoint: "https://dmcr.example.com/direct",
		ModelPackPublisher:      fakePublisher{},
	})
	if err == nil || !strings.Contains(err.Error(), "source-url is required") {
		t.Fatalf("Run() error = %v, want missing source-url", err)
	}
}

type fakeRemoteObjectReader struct{}

func (fakeRemoteObjectReader) OpenRead(context.Context, string) (sourcefetch.RemoteOpenReadResult, error) {
	return sourcefetch.RemoteOpenReadResult{SizeBytes: 42}, nil
}
