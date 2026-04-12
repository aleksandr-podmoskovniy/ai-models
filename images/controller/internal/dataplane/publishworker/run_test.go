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
	"archive/tar"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type fakePublisher struct{}

func (fakePublisher) Publish(context.Context, modelpackports.PublishInput, modelpackports.RegistryAuth) (modelpackports.PublishResult, error) {
	return modelpackports.PublishResult{
		Reference: "registry.example.com/ai-models/catalog/model@sha256:deadbeef",
		Digest:    "sha256:deadbeef",
		MediaType: "application/vnd.cncf.model.manifest.v1+json",
		SizeBytes: 123,
	}, nil
}

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
		RuntimeEngines:     []string{"KServe"},
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
		RuntimeEngines:     []string{"KServe"},
		ModelPackPublisher: fakePublisher{},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := result.Resolved.Task, "text-generation"; got != want {
		t.Fatalf("unexpected resolved task %q", got)
	}
}

func TestPublishFromUploadRejectsForbiddenFile(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tar")
	if err := createTestTar(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"model_type":"qwen3"}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
		tarEntry{name: "checkpoint/model.py", content: []byte("print('boom')")},
	); err != nil {
		t.Fatalf("createTestTar() error = %v", err)
	}

	_, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         archivePath,
		InputFormat:        modelsv1alpha1.ModelInputFormatSafetensors,
		Task:               "text-generation",
		RuntimeEngines:     []string{"KServe"},
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

	result, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         modelPath,
		Task:               "text-generation",
		RuntimeEngines:     []string{"KubeRay"},
		ModelPackPublisher: fakePublisher{},
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

func TestPublishFromHTTPAcceptsDirectGGUFFile(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/model.gguf" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte("GGUFpayload"))
	}))
	defer server.Close()

	result, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeHTTP,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		HTTPURL:            server.URL + "/model.gguf",
		Task:               "text-generation",
		RuntimeEngines:     []string{"KubeRay"},
		ModelPackPublisher: fakePublisher{},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := result.Resolved.Format, "GGUF"; got != want {
		t.Fatalf("unexpected resolved format %q", got)
	}
	if got, want := result.Source.Type, modelsv1alpha1.ModelSourceTypeHTTP; got != want {
		t.Fatalf("unexpected source type %q", got)
	}
}

func TestEnsureWorkspaceCreatesAndCleansSubdirectoryUnderSnapshotRoot(t *testing.T) {
	t.Parallel()

	snapshotRoot := t.TempDir()
	workspace, cleanupDir, err := ensureWorkspace(snapshotRoot, "ai-model-publish-")
	if err != nil {
		t.Fatalf("ensureWorkspace() error = %v", err)
	}
	if got, want := filepath.Dir(workspace), snapshotRoot; got != want {
		t.Fatalf("unexpected workspace parent %q", got)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("Stat(workspace) error = %v", err)
	}

	cleanupDir()

	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("expected workspace cleanup, got err=%v", err)
	}
	if _, err := os.Stat(snapshotRoot); err != nil {
		t.Fatalf("expected snapshot root to remain, got err=%v", err)
	}
}

func TestAttachResolvedProfileProvenance(t *testing.T) {
	t.Parallel()

	resolved := attachResolvedProfileProvenance(publicationdata.ResolvedProfile{
		Task:   "text-generation",
		Format: "Safetensors",
	}, sourceProfileProvenance{
		License:      "apache-2.0",
		SourceRepoID: "deepseek-ai/DeepSeek-R1",
	})

	if got, want := resolved.License, "apache-2.0"; got != want {
		t.Fatalf("unexpected license %q", got)
	}
	if got, want := resolved.SourceRepoID, "deepseek-ai/DeepSeek-R1"; got != want {
		t.Fatalf("unexpected source repo ID %q", got)
	}
}

type tarEntry struct {
	name    string
	content []byte
}

func createTestTar(path string, entries ...tarEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := tar.NewWriter(file)
	defer writer.Close()

	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.content))}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if _, err := writer.Write(entry.content); err != nil {
			return err
		}
	}
	return nil
}
