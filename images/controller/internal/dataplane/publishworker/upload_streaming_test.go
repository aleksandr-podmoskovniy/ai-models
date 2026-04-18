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
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestPublishFromUploadStreamsTarArchiveIntoPublisher(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		fileName    string
		create      func(string) error
		compression modelpackports.LayerCompression
	}{
		{
			name:     "tar",
			fileName: "checkpoint.tar",
			create: func(path string) error {
				return createTestTar(path,
					tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
					tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
					tarEntry{name: "checkpoint/model.py", content: []byte("print('helper')")},
				)
			},
			compression: modelpackports.LayerCompressionNone,
		},
		{
			name:     "zip",
			fileName: "checkpoint.zip",
			create: func(path string) error {
				return createTestZip(path,
					tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
					tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
					tarEntry{name: "checkpoint/model.py", content: []byte("print('helper')")},
				)
			},
			compression: modelpackports.LayerCompressionNone,
		},
		{
			name:     "tar.zst",
			fileName: "checkpoint.tar.zst",
			create: func(path string) error {
				return createTestZstdTar(path,
					tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
					tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
					tarEntry{name: "checkpoint/model.py", content: []byte("print('helper')")},
				)
			},
			compression: modelpackports.LayerCompressionZstd,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			archivePath := filepath.Join(t.TempDir(), tc.fileName)
			if err := tc.create(archivePath); err != nil {
				t.Fatalf("create archive error = %v", err)
			}

			publisher := fakePublisher{
				onPublish: func(input modelpackports.PublishInput) error {
					if got, want := len(input.Layers), 1; got != want {
						t.Fatalf("unexpected layer count %d", got)
					}
					layer := input.Layers[0]
					if layer.Archive == nil {
						t.Fatal("expected archive streaming layer")
					}
					if got, want := layer.Base, modelpackports.LayerBaseModel; got != want {
						t.Fatalf("unexpected layer base %q", got)
					}
					if got, want := layer.Format, modelpackports.LayerFormatTar; got != want {
						t.Fatalf("unexpected layer format %q", got)
					}
					if got, want := layer.Compression, tc.compression; got != want {
						t.Fatalf("unexpected layer compression %q", got)
					}
					if got, want := layer.Archive.StripPathPrefix, "checkpoint"; got != want {
						t.Fatalf("unexpected strip prefix %q", got)
					}
					if got, want := layer.Archive.SelectedFiles, []string{"config.json", "model.safetensors"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
						t.Fatalf("unexpected selected files %#v", got)
					}
					return nil
				},
			}

			result, err := run(context.Background(), Options{
				SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
				ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
				UploadPath:         archivePath,
				InputFormat:        modelsv1alpha1.ModelInputFormatSafetensors,
				Task:               "text-generation",
				ModelPackPublisher: publisher,
			})
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if got, want := result.Resolved.Format, "Safetensors"; got != want {
				t.Fatalf("unexpected resolved format %q", got)
			}
		})
	}
}

func TestPublishFromUploadStreamsArchiveWrappedGGUFIntoPublisher(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		fileName string
		create   func(string) error
	}{
		{
			name:     "tar",
			fileName: "bundle.tar",
			create: func(path string) error {
				return createTestTar(path,
					tarEntry{name: "bundle/deepseek-r1-8b-q4_k_m.gguf", content: []byte("GGUFweights")},
					tarEntry{name: "bundle/README.md", content: []byte("# docs")},
				)
			},
		},
		{
			name:     "zip",
			fileName: "bundle.zip",
			create: func(path string) error {
				return createTestZip(path,
					tarEntry{name: "bundle/deepseek-r1-8b-q4_k_m.gguf", content: []byte("GGUFweights")},
					tarEntry{name: "bundle/README.md", content: []byte("# docs")},
				)
			},
		},
		{
			name:     "tar.zst",
			fileName: "bundle.tar.zst",
			create: func(path string) error {
				return createTestZstdTar(path,
					tarEntry{name: "bundle/deepseek-r1-8b-q4_k_m.gguf", content: []byte("GGUFweights")},
					tarEntry{name: "bundle/README.md", content: []byte("# docs")},
				)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			archivePath := filepath.Join(t.TempDir(), tc.fileName)
			if err := tc.create(archivePath); err != nil {
				t.Fatalf("create archive error = %v", err)
			}

			publisher := fakePublisher{
				onPublish: func(input modelpackports.PublishInput) error {
					if got, want := len(input.Layers), 1; got != want {
						t.Fatalf("unexpected layer count %d", got)
					}
					layer := input.Layers[0]
					if layer.Archive == nil {
						t.Fatal("expected archive streaming layer")
					}
					if got, want := layer.Archive.StripPathPrefix, "bundle"; got != want {
						t.Fatalf("unexpected strip prefix %q", got)
					}
					if got, want := layer.Archive.SelectedFiles, []string{"deepseek-r1-8b-q4_k_m.gguf"}; len(got) != len(want) || got[0] != want[0] {
						t.Fatalf("unexpected selected files %#v", got)
					}
					return nil
				},
			}

			result, err := run(context.Background(), Options{
				SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
				ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
				UploadPath:         archivePath,
				InputFormat:        modelsv1alpha1.ModelInputFormatGGUF,
				Task:               "text-generation",
				ModelPackPublisher: publisher,
			})
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if got, want := result.Resolved.Format, "GGUF"; got != want {
				t.Fatalf("unexpected resolved format %q", got)
			}
			if got, want := result.Resolved.Family, "deepseek-r1"; got != want {
				t.Fatalf("unexpected resolved family %q", got)
			}
		})
	}
}
