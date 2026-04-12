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

package sourcefetch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHuggingFaceHTTPSnapshotDownloaderResolveURL(t *testing.T) {
	t.Parallel()

	downloader := &huggingFaceHTTPSnapshotDownloader{BaseURL: "https://huggingface.example.com"}
	got, err := downloader.resolveURL("owner/model", "refs/pr/1", "sub dir/model.safetensors")
	if err != nil {
		t.Fatalf("resolveURL() error = %v", err)
	}

	want := "https://huggingface.example.com/owner/model/resolve/refs/pr/1/sub%20dir/model.safetensors"
	if got != want {
		t.Fatalf("unexpected resolve URL %q", got)
	}
}

func TestHuggingFaceHTTPSnapshotDownloaderDownload(t *testing.T) {
	t.Parallel()

	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authHeader = request.Header.Get("Authorization")
		switch request.URL.Path {
		case "/owner/model/resolve/deadbeef/config.json":
			_, _ = writer.Write([]byte(`{"architectures":["LlamaForCausalLM"]}`))
		case "/owner/model/resolve/deadbeef/model.safetensors":
			_, _ = writer.Write([]byte("tensor-payload"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	workspace := t.TempDir()
	downloader := &huggingFaceHTTPSnapshotDownloader{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}
	if err := downloader.Download(t.Context(), huggingFaceSnapshotDownloadInput{
		RepoID:      "owner/model",
		Revision:    "deadbeef",
		Token:       "hf-token",
		Files:       []string{"config.json", "model.safetensors"},
		SnapshotDir: workspace,
	}); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if got, want := authHeader, "Bearer hf-token"; got != want {
		t.Fatalf("unexpected authorization header %q", got)
	}
	if payload, err := os.ReadFile(filepath.Join(workspace, "config.json")); err != nil {
		t.Fatalf("ReadFile(config.json) error = %v", err)
	} else if got, want := string(payload), `{"architectures":["LlamaForCausalLM"]}`; got != want {
		t.Fatalf("unexpected config payload %q", got)
	}
	if payload, err := os.ReadFile(filepath.Join(workspace, "model.safetensors")); err != nil {
		t.Fatalf("ReadFile(model.safetensors) error = %v", err)
	} else if got, want := string(payload), "tensor-payload"; got != want {
		t.Fatalf("unexpected model payload %q", got)
	}
}

func TestValidateHuggingFaceSnapshotDownloadInput(t *testing.T) {
	t.Parallel()

	err := validateHuggingFaceSnapshotDownloadInput(huggingFaceSnapshotDownloadInput{
		RepoID:      "owner/model",
		Revision:    "deadbeef",
		Files:       []string{"config.json"},
		SnapshotDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("validateHuggingFaceSnapshotDownloadInput() error = %v", err)
	}
}
