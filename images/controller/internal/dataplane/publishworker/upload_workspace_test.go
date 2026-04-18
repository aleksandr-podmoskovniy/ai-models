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

func TestPublishFromUploadDirectGGUFDoesNotCreateWorkspace(t *testing.T) {
	t.Parallel()

	modelPath := writeTempFile(t, "model.gguf", []byte("GGUFpayload"))
	publisher := fakePublisher{
		onPublish: func(input modelpackports.PublishInput) error {
			if got, want := input.ModelDir, modelPath; got != want {
				t.Fatalf("unexpected publish input path %q", got)
			}
			return nil
		},
	}

	_, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         modelPath,
		Task:               "text-generation",
		ModelPackPublisher: publisher,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestPublishFromUploadStreamingArchiveDoesNotCreateWorkspace(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tar")
	if err := createTestTar(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
	); err != nil {
		t.Fatalf("createTestTar() error = %v", err)
	}

	publisher := fakePublisher{
		onPublish: func(input modelpackports.PublishInput) error {
			if got, want := len(input.Layers), 1; got != want {
				t.Fatalf("unexpected layer count %d", got)
			}
			if input.Layers[0].Archive == nil {
				t.Fatal("expected archive streaming layer")
			}
			return nil
		},
	}

	_, err := run(context.Background(), Options{
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
}
