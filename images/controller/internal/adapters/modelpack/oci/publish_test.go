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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestAdapterPublishMaterializeAndRemove(t *testing.T) {
	t.Parallel()

	server, auth := newWritableRegistryServer(t)
	defer server.Close()

	modelDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(modelDir, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "config.json"), []byte("{\"family\":\"tiny\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "nested", "weights.gguf"), []byte("GGUF-test"), 0o644); err != nil {
		t.Fatalf("WriteFile(weights.gguf) error = %v", err)
	}

	adapter := New()
	reference := serverReference(server.server, "published")
	publishResult, err := adapter.Publish(context.Background(), modelpackports.PublishInput{
		ModelDir:    modelDir,
		ArtifactURI: reference,
	}, auth)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if got := publishResult.Reference; got != immutableOCIReference(reference, publishResult.Digest) {
		t.Fatalf("Publish().Reference = %q, want immutable reference", got)
	}
	if publishResult.MediaType != ModelPackArtifactType {
		t.Fatalf("Publish().MediaType = %q, want %q", publishResult.MediaType, ModelPackArtifactType)
	}
	if publishResult.SizeBytes <= 0 {
		t.Fatalf("Publish().SizeBytes = %d, want positive size", publishResult.SizeBytes)
	}
	if server.patchCount() == 0 {
		t.Fatalf("Publish() must stream the layer via PATCH upload")
	}

	payload, err := InspectRemote(context.Background(), reference, auth)
	if err != nil {
		t.Fatalf("InspectRemote() error = %v", err)
	}
	if err := ValidatePayload(payload); err != nil {
		t.Fatalf("ValidatePayload() error = %v", err)
	}

	materializeDir := filepath.Join(t.TempDir(), "materialized")
	materialized, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    reference,
		ArtifactDigest: publishResult.Digest,
		DestinationDir: materializeDir,
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if got := filepath.Clean(materialized.ModelPath); got != filepath.Join(materializeDir, materializedLayerPath) {
		t.Fatalf("Materialize().ModelPath = %q, want contract path", got)
	}

	assertFileContent(t, filepath.Join(materialized.ModelPath, "config.json"), "{\"family\":\"tiny\"}\n")
	assertFileContent(t, filepath.Join(materialized.ModelPath, "nested", "weights.gguf"), "GGUF-test")

	if err := adapter.Remove(context.Background(), publishResult.Reference, auth); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := InspectRemote(context.Background(), reference, auth); err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("InspectRemote() after Remove() error = %v, want 404", err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if got := string(body); got != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

type writableRegistryServer struct {
	server        *httptest.Server
	getPatchCount func() int
}

func (s *writableRegistryServer) Close() {
	s.server.Close()
}

func (s *writableRegistryServer) patchCount() int {
	return s.getPatchCount()
}

func newWritableRegistryServer(t *testing.T) (*writableRegistryServer, modelpackports.RegistryAuth) {
	t.Helper()

	type registryState struct {
		uploads   map[string][]byte
		blobs     map[string][]byte
		manifests map[string][]byte
		tags      map[string]string
		patches   int
	}
	state := registryState{
		uploads:   make(map[string][]byte),
		blobs:     make(map[string][]byte),
		manifests: make(map[string][]byte),
		tags:      make(map[string]string),
	}

	const repoPrefix = "/v2/ai-models/catalog/model"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "writer" || pass != "secret" {
			t.Fatalf("unexpected auth %q/%q", user, pass)
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == repoPrefix+"/blobs/uploads/":
			uploadID := "upload-" + hex.EncodeToString([]byte{byte(len(state.uploads) + 1)})
			state.uploads[uploadID] = nil
			w.Header().Set("Location", "/uploads/"+uploadID)
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/uploads/"):
			uploadID := strings.TrimPrefix(r.URL.Path, "/uploads/")
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(patch body) error = %v", err)
			}
			state.uploads[uploadID] = append(state.uploads[uploadID], payload...)
			state.patches++
			w.Header().Set("Location", "/uploads/"+uploadID)
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/uploads/"):
			uploadID := strings.TrimPrefix(r.URL.Path, "/uploads/")
			digest := strings.TrimSpace(r.URL.Query().Get("digest"))
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(upload body) error = %v", err)
			}
			if uploadID == "" || digest == "" {
				t.Fatalf("unexpected upload request %q?%q", r.URL.Path, r.URL.RawQuery)
			}
			state.blobs[digest] = append(state.uploads[uploadID], payload...)
			delete(state.uploads, uploadID)
			w.WriteHeader(http.StatusCreated)
		case strings.HasPrefix(r.URL.Path, repoPrefix+"/manifests/"):
			ref := strings.TrimPrefix(r.URL.Path, repoPrefix+"/manifests/")
			switch r.Method {
			case http.MethodPut:
				payload, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("ReadAll(manifest body) error = %v", err)
				}
				digestBytes := sha256.Sum256(payload)
				digest := "sha256:" + hex.EncodeToString(digestBytes[:])
				state.manifests[digest] = payload
				if !strings.HasPrefix(ref, "sha256:") {
					state.tags[ref] = digest
				}
				w.Header().Set("Docker-Content-Digest", digest)
				w.WriteHeader(http.StatusCreated)
			case http.MethodGet:
				digest := ref
				if !strings.HasPrefix(digest, "sha256:") {
					digest = state.tags[ref]
				}
				payload, ok := state.manifests[digest]
				if !ok {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Docker-Content-Digest", digest)
				w.Header().Set("Content-Type", ManifestMediaType)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(payload)
			case http.MethodDelete:
				delete(state.manifests, ref)
				for tag, digest := range state.tags {
					if digest == ref {
						delete(state.tags, tag)
					}
				}
				w.WriteHeader(http.StatusAccepted)
			default:
				t.Fatalf("unexpected manifest method %s", r.Method)
			}
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, repoPrefix+"/blobs/"):
			digest := strings.TrimPrefix(r.URL.Path, repoPrefix+"/blobs/")
			payload, ok := state.blobs[digest]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		default:
			t.Fatalf("unexpected path %s %q", r.Method, r.URL.Path)
		}
	}))

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	return &writableRegistryServer{
			server: server,
			getPatchCount: func() int {
				return state.patches
			},
		}, modelpackports.RegistryAuth{
			Username: "writer",
			Password: "secret",
			CAFile:   writeTempFile(t, certPEM),
		}
}
