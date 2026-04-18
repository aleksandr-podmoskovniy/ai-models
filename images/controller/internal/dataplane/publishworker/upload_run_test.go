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
	"os"
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestPublishFromUploadBuildsBackendResult(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tar")
	if err := createTestTar(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"model_type":"qwen3","architectures":["Qwen3ForCausalLM"],"torch_dtype":"bfloat16","text_config":{"hidden_size":4096,"intermediate_size":11008,"num_hidden_layers":32,"num_attention_heads":32,"num_key_value_heads":8,"max_position_embeddings":32768,"vocab_size":151936}}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
	); err != nil {
		t.Fatalf("createTestTar() error = %v", err)
	}

	result, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         archivePath,
		InputFormat:        modelsv1alpha1.ModelInputFormatSafetensors,
		Task:               "text-generation",
		ModelPackPublisher: fakePublisher{},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := result.Artifact.URI, "registry.example.com/ai-models/catalog/model@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected artifact URI %q", got)
	}
	if got, want := result.Resolved.SourceRepoID, ""; got != want {
		t.Fatalf("unexpected source repo ID %q", got)
	}
	if result.Resolved.ParameterCount <= 0 {
		t.Fatalf("expected parameter count, got %#v", result.Resolved)
	}
}

func TestPublishFromUploadInfersSafetensorsTaskFromCheckpoint(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tar")
	if err := createTestTar(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"model_type":"qwen3","architectures":["Qwen3ForCausalLM"],"torch_dtype":"bfloat16","text_config":{"hidden_size":4096,"intermediate_size":11008,"num_hidden_layers":32,"num_attention_heads":32,"num_key_value_heads":8,"max_position_embeddings":32768,"vocab_size":151936}}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
	); err != nil {
		t.Fatalf("createTestTar() error = %v", err)
	}

	result, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         archivePath,
		InputFormat:        modelsv1alpha1.ModelInputFormatSafetensors,
		ModelPackPublisher: fakePublisher{},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := result.Resolved.Task, "text-generation"; got != want {
		t.Fatalf("unexpected resolved task %q", got)
	}
}

func TestPublishFromUploadDropsHelperScript(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tar")
	if err := createTestTar(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"model_type":"qwen3","architectures":["Qwen3ForCausalLM"],"torch_dtype":"bfloat16","text_config":{"hidden_size":2,"intermediate_size":2,"num_hidden_layers":1,"num_attention_heads":1,"num_key_value_heads":1,"max_position_embeddings":16,"vocab_size":32}}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
		tarEntry{name: "checkpoint/model.py", content: []byte("print('boom')")},
	); err != nil {
		t.Fatalf("createTestTar() error = %v", err)
	}

	result, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         archivePath,
		InputFormat:        modelsv1alpha1.ModelInputFormatSafetensors,
		Task:               "text-generation",
		ModelPackPublisher: fakePublisher{},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := result.Resolved.Format, "Safetensors"; got != want {
		t.Fatalf("unexpected resolved format %q", got)
	}
}

func TestPublishFromUploadRejectsCompiledPayload(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tar")
	if err := createTestTar(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"model_type":"qwen3"}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
		tarEntry{name: "checkpoint/libpayload.so", content: []byte("boom")},
	); err != nil {
		t.Fatalf("createTestTar() error = %v", err)
	}

	_, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         archivePath,
		InputFormat:        modelsv1alpha1.ModelInputFormatSafetensors,
		Task:               "text-generation",
		ModelPackPublisher: fakePublisher{},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestPublishFromUploadAcceptsDirectGGUFFile(t *testing.T) {
	t.Parallel()

	modelPath := filepath.Join(t.TempDir(), "model")
	if err := os.WriteFile(modelPath, []byte("GGUFpayload"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	publisher := fakePublisher{
		onPublish: func(input modelpackports.PublishInput) error {
			if got, want := input.ModelDir, modelPath; got != want {
				t.Fatalf("unexpected publish input path %q", got)
			}
			return nil
		},
	}

	result, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         modelPath,
		Task:               "text-generation",
		ModelPackPublisher: publisher,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := result.Resolved.Format, "GGUF"; got != want {
		t.Fatalf("unexpected resolved format %q", got)
	}
	if got, want := result.Artifact.Digest, "sha256:deadbeef"; got != want {
		t.Fatalf("unexpected artifact digest %q", got)
	}
}
