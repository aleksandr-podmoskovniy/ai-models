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

package main

import (
	"archive/tar"
	"bytes"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
)

func TestRunMaterializeArtifactFromEnv(t *testing.T) {
	layerTar := tarBytes(t, map[string]string{
		"model/config.json": "{}",
	})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/ai-models/catalog/model/manifests/sha256:deadbeef":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "reader" || pass != "secret" {
				t.Fatalf("unexpected auth %q/%q", user, pass)
			}
			w.Header().Set("Docker-Content-Digest", "sha256:deadbeef")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"schemaVersion":2,"artifactType":"application/vnd.cncf.model.manifest.v1+json","config":{"mediaType":"application/vnd.cncf.model.config.v1+json","digest":"sha256:config","size":10},"layers":[{"mediaType":"application/vnd.cncf.model.weight.v1.tar","digest":"sha256:layer","size":30,"annotations":{"org.cncf.model.filepath":"model"}}]}`))
		case "/v2/ai-models/catalog/model/blobs/sha256:config":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "reader" || pass != "secret" {
				t.Fatalf("unexpected auth %q/%q", user, pass)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"descriptor":{"name":"model"},"modelfs":{"type":"layers","diffIds":["sha256:layer-diff"]},"config":{}}`))
		case "/v2/ai-models/catalog/model/blobs/sha256:layer":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "reader" || pass != "secret" {
				t.Fatalf("unexpected auth %q/%q", user, pass)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(layerTar)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	caFile := writePEMFile(t, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	destination := filepath.Join(t.TempDir(), "current")

	t.Setenv("AI_MODELS_MATERIALIZE_ARTIFACT_URI", strings.TrimPrefix(server.URL, "https://")+"/ai-models/catalog/model@sha256:deadbeef")
	t.Setenv("AI_MODELS_MATERIALIZE_ARTIFACT_DIGEST", "sha256:deadbeef")
	t.Setenv("AI_MODELS_MATERIALIZE_DESTINATION_DIR", destination)
	t.Setenv("AI_MODELS_MATERIALIZE_ARTIFACT_FAMILY", "hf-safetensors-v1")
	t.Setenv("AI_MODELS_OCI_USERNAME", "reader")
	t.Setenv("AI_MODELS_OCI_PASSWORD", "secret")
	t.Setenv("AI_MODELS_OCI_CA_FILE", caFile)

	if exitCode := runMaterializeArtifact(nil); exitCode != 0 {
		t.Fatalf("runMaterializeArtifact() exitCode = %d", exitCode)
	}
	if _, err := os.Stat(filepath.Join(destination, "model", "config.json")); err != nil {
		t.Fatalf("expected materialized config.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, ".ai-models-materialized.json")); err != nil {
		t.Fatalf("expected materialization marker: %v", err)
	}
}

func TestRunMaterializeArtifactSupportsCacheRoot(t *testing.T) {
	layerTar := tarBytes(t, map[string]string{
		"model/config.json": "{}",
	})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/ai-models/catalog/model/manifests/sha256:deadbeef":
			w.Header().Set("Docker-Content-Digest", "sha256:deadbeef")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"schemaVersion":2,"artifactType":"application/vnd.cncf.model.manifest.v1+json","config":{"mediaType":"application/vnd.cncf.model.config.v1+json","digest":"sha256:config","size":10},"layers":[{"mediaType":"application/vnd.cncf.model.weight.v1.tar","digest":"sha256:layer","size":30,"annotations":{"org.cncf.model.filepath":"model"}}]}`))
		case "/v2/ai-models/catalog/model/blobs/sha256:config":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"descriptor":{"name":"model"},"modelfs":{"type":"layers","diffIds":["sha256:layer-diff"]},"config":{}}`))
		case "/v2/ai-models/catalog/model/blobs/sha256:layer":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(layerTar)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	caFile := writePEMFile(t, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	cacheRoot := filepath.Join(t.TempDir(), "modelcache")

	t.Setenv("AI_MODELS_MATERIALIZE_ARTIFACT_URI", strings.TrimPrefix(server.URL, "https://")+"/ai-models/catalog/model@sha256:deadbeef")
	t.Setenv("AI_MODELS_MATERIALIZE_CACHE_ROOT", cacheRoot)
	t.Setenv("AI_MODELS_OCI_USERNAME", "reader")
	t.Setenv("AI_MODELS_OCI_PASSWORD", "secret")
	t.Setenv("AI_MODELS_OCI_CA_FILE", caFile)

	if exitCode := runMaterializeArtifact(nil); exitCode != 0 {
		t.Fatalf("runMaterializeArtifact() exitCode = %d", exitCode)
	}

	storePath := filepath.Join(cacheRoot, "store", "sha256:deadbeef", "model", "config.json")
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected stored model config.json: %v", err)
	}
	currentPath := filepath.Join(cacheRoot, "current")
	linkTarget, err := os.Readlink(currentPath)
	if err != nil {
		t.Fatalf("Readlink(current) error = %v", err)
	}
	if got, want := linkTarget, filepath.Join("store", "sha256:deadbeef", "model"); got != want {
		t.Fatalf("current target = %q, want %q", got, want)
	}
	modelPath := filepath.Join(cacheRoot, "model")
	modelTarget, err := os.Readlink(modelPath)
	if err != nil {
		t.Fatalf("Readlink(model) error = %v", err)
	}
	if got, want := modelTarget, "current"; got != want {
		t.Fatalf("model target = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(modelPath, "config.json")); err != nil {
		t.Fatalf("expected config.json through workload model symlink: %v", err)
	}
}

func TestRunMaterializeArtifactUsesSharedStoreWithoutGlobalSymlinks(t *testing.T) {
	layerTar := tarBytes(t, map[string]string{
		"model/config.json": "{}",
	})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/ai-models/catalog/model/manifests/sha256:deadbeef":
			w.Header().Set("Docker-Content-Digest", "sha256:deadbeef")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"schemaVersion":2,"artifactType":"application/vnd.cncf.model.manifest.v1+json","config":{"mediaType":"application/vnd.cncf.model.config.v1+json","digest":"sha256:config","size":10},"layers":[{"mediaType":"application/vnd.cncf.model.weight.v1.tar","digest":"sha256:layer","size":30,"annotations":{"org.cncf.model.filepath":"model"}}]}`))
		case "/v2/ai-models/catalog/model/blobs/sha256:config":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"descriptor":{"name":"model"},"modelfs":{"type":"layers","diffIds":["sha256:layer-diff"]},"config":{}}`))
		case "/v2/ai-models/catalog/model/blobs/sha256:layer":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(layerTar)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	caFile := writePEMFile(t, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	cacheRoot := filepath.Join(t.TempDir(), "modelcache")

	t.Setenv("AI_MODELS_MATERIALIZE_ARTIFACT_URI", strings.TrimPrefix(server.URL, "https://")+"/ai-models/catalog/model@sha256:deadbeef")
	t.Setenv("AI_MODELS_MATERIALIZE_CACHE_ROOT", cacheRoot)
	t.Setenv("AI_MODELS_MATERIALIZE_SHARED_STORE", "true")
	t.Setenv("AI_MODELS_MATERIALIZE_MODEL_NAME", "main")
	t.Setenv("AI_MODELS_OCI_USERNAME", "reader")
	t.Setenv("AI_MODELS_OCI_PASSWORD", "secret")
	t.Setenv("AI_MODELS_OCI_CA_FILE", caFile)

	if exitCode := runMaterializeArtifact(nil); exitCode != 0 {
		t.Fatalf("runMaterializeArtifact() exitCode = %d", exitCode)
	}

	if _, err := os.Stat(filepath.Join(nodecache.SharedArtifactModelPath(cacheRoot, "sha256:deadbeef"), "config.json")); err != nil {
		t.Fatalf("expected config.json through shared artifact model path: %v", err)
	}
	modelPath := nodecache.WorkloadNamedModelPath(cacheRoot, "main")
	if _, err := os.Stat(filepath.Join(modelPath, "config.json")); err != nil {
		t.Fatalf("expected config.json through named model path: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(cacheRoot, "current")); !os.IsNotExist(err) {
		t.Fatalf("did not expect current symlink in shared-store mode, stat err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(cacheRoot, "model")); !os.IsNotExist(err) {
		t.Fatalf("did not expect model symlink in shared-store mode, stat err=%v", err)
	}
}

func tarBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader() error = %v", err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buffer.Bytes()
}

func writePEMFile(t *testing.T, content []byte) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "ca-*.pem")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return file.Name()
}
