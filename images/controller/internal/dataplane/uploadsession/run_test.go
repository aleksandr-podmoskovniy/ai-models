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

package uploadsession

import (
	"archive/tar"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/dataplane/publishworker"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestRunRejectsMissingUploadToken(t *testing.T) {
	t.Parallel()

	_, err := Run(t.Context(), Options{
		Publish: publishworker.Options{
			Task: "text-generation",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "upload token") {
		t.Fatalf("expected upload token validation error, got %v", err)
	}
}

func TestRunRejectsMissingTask(t *testing.T) {
	t.Parallel()

	_, err := Run(t.Context(), Options{
		UploadToken: "token",
	})
	if err == nil || !strings.Contains(err.Error(), "task is required") {
		t.Fatalf("expected task validation error, got %v", err)
	}
}

func TestHandlerExposesHealthz(t *testing.T) {
	t.Parallel()

	handler := newHandler(t.TempDir(), Options{
		UploadToken: "token",
		Publish: publishworker.Options{
			Task: "text-generation",
		},
	}, make(chan runResult, 1))

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("unexpected status %d", got)
	}
}

func TestHandlerRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	handler := newHandler(t.TempDir(), Options{
		UploadToken: "token",
		InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
		Publish: publishworker.Options{
			Task: "text-generation",
		},
	}, make(chan runResult, 1))

	request := httptest.NewRequest(http.MethodPut, "/upload", strings.NewReader("payload"))
	request.Header.Set("Authorization", "Bearer wrong")
	request.Header.Set("Content-Length", "7")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("unexpected status %d", got)
	}
}

func TestHandlerPreservesUploadedArchiveFileName(t *testing.T) {
	t.Parallel()

	handler := newHandler(t.TempDir(), Options{
		UploadToken: "token",
		InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
		Publish: publishworker.Options{
			ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
			Task:               "text-generation",
			RuntimeEngines:     []string{"KServe"},
			ModelPackPublisher: fakePublisher{},
		},
	}, make(chan runResult, 1))

	body := buildTarArchive(t,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"model_type":"qwen3","architectures":["Qwen3ForCausalLM"],"torch_dtype":"bfloat16","text_config":{"hidden_size":4096,"intermediate_size":11008,"num_hidden_layers":32,"num_attention_heads":32,"num_key_value_heads":8,"max_position_embeddings":32768,"vocab_size":151936}}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
	)

	request := httptest.NewRequest(http.MethodPut, "/upload", bytes.NewReader(body))
	request = request.WithContext(context.Background())
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Length", strconv.Itoa(len(body)))
	request.Header.Set(uploadFilenameHeader, "model.tar")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
}

func TestHandlerAcceptsDirectGGUFFile(t *testing.T) {
	t.Parallel()

	handler := newHandler(t.TempDir(), Options{
		UploadToken: "token",
		Publish: publishworker.Options{
			ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
			Task:               "text-generation",
			RuntimeEngines:     []string{"KubeRay"},
			ModelPackPublisher: fakePublisher{},
		},
	}, make(chan runResult, 1))

	body := []byte("GGUFpayload")

	request := httptest.NewRequest(http.MethodPut, "/upload", bytes.NewReader(body))
	request = request.WithContext(context.Background())
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Length", strconv.Itoa(len(body)))
	request.Header.Set(uploadFilenameHeader, "model.gguf")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
}

func TestSanitizedUploadFileName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "upload.bin"},
		{name: "basename", input: "model.tar.gz", want: "model.tar.gz"},
		{name: "path", input: "/tmp/model.gguf", want: "model.gguf"},
		{name: "windows path", input: `C:\tmp\model.gguf`, want: "model.gguf"},
		{name: "hidden", input: ".env", want: "upload.bin"},
		{name: "parent", input: "../evil.tar", want: "evil.tar"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizedUploadFileName(tc.input); got != tc.want {
				t.Fatalf("sanitizedUploadFileName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizePortDefaults(t *testing.T) {
	t.Parallel()

	if got, want := normalizePort(0), 8444; got != want {
		t.Fatalf("normalizePort(0) = %d, want %d", got, want)
	}
	if got, want := normalizePort(18080), 18080; got != want {
		t.Fatalf("normalizePort(18080) = %d, want %d", got, want)
	}
}

type fakePublisher struct{}

func (fakePublisher) Publish(context.Context, modelpackports.PublishInput, modelpackports.RegistryAuth) (modelpackports.PublishResult, error) {
	return modelpackports.PublishResult{
		Reference: "registry.example.com/ai-models/catalog/model@sha256:deadbeef",
		Digest:    "sha256:deadbeef",
		MediaType: "application/vnd.cncf.model.manifest.v1+json",
		SizeBytes: 123,
	}, nil
}

type tarEntry struct {
	name    string
	content []byte
}

func buildTarArchive(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()

	path := filepath.Join(t.TempDir(), "model.tar")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	writer := tar.NewWriter(file)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.content))}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader() error = %v", err)
		}
		if _, err := writer.Write(entry.content); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return payload
}
