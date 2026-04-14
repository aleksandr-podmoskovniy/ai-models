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
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestMaterializerMaterializeAndReuse(t *testing.T) {
	t.Parallel()

	var layerRequests atomic.Int32
	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		layerTar: tarBytes(t, map[string]string{
			"model/config.json":                  `{"model_type":"phi"}`,
			"model/tokenizer_config.json":        `{"tokenizer_class":"AutoTokenizer"}`,
			"model/model.safetensors.index.json": `{"metadata":{}}`,
		}),
		layerHook: func() {
			layerRequests.Add(1)
		},
	})
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "current")
	result, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "published"),
		ArtifactDigest: "sha256:deadbeef",
		DestinationDir: destination,
		ArtifactFamily: "hf-safetensors-v1",
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	if got, want := result.ModelPath, filepath.Join(destination, "model"); got != want {
		t.Fatalf("unexpected model path %q", got)
	}
	if got, want := result.MediaType, ModelPackArtifactType; got != want {
		t.Fatalf("unexpected media type %q", got)
	}
	if got, want := layerRequests.Load(), int32(1); got != want {
		t.Fatalf("unexpected layer request count %d", got)
	}
	if _, err := os.Stat(filepath.Join(result.ModelPath, "config.json")); err != nil {
		t.Fatalf("expected materialized config.json: %v", err)
	}
	markerBody, err := os.ReadFile(result.MarkerPath)
	if err != nil {
		t.Fatalf("ReadFile(marker) error = %v", err)
	}
	if !strings.Contains(string(markerBody), "\"family\": \"hf-safetensors-v1\"") {
		t.Fatalf("marker is missing family: %s", string(markerBody))
	}

	reused, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "published"),
		ArtifactDigest: "sha256:deadbeef",
		DestinationDir: destination,
		ArtifactFamily: "hf-safetensors-v1",
	}, auth)
	if err != nil {
		t.Fatalf("Materialize(reuse) error = %v", err)
	}
	if reused.ModelPath != result.ModelPath {
		t.Fatalf("unexpected reused model path %q", reused.ModelPath)
	}
	if got, want := layerRequests.Load(), int32(1); got != want {
		t.Fatalf("expected layer not to be fetched again, got %d requests", got)
	}
}

func TestMaterializerAcceptsImmutableDigestReference(t *testing.T) {
	t.Parallel()

	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		layerTar: tarBytes(t, map[string]string{
			"model/model.gguf": "GGUF",
		}),
	})
	defer server.Close()

	result, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: filepath.Join(t.TempDir(), "current"),
		ArtifactFamily: "gguf-v1",
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if got, want := result.Digest, "sha256:deadbeef"; got != want {
		t.Fatalf("unexpected digest %q", got)
	}
	if _, err := os.Stat(result.ModelPath); err != nil {
		t.Fatalf("expected materialized model path: %v", err)
	}
}

func TestMaterializerRejectsDigestMismatch(t *testing.T) {
	t.Parallel()

	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		layerTar: tarBytes(t, map[string]string{"model/model.gguf": "GGUF"}),
	})
	defer server.Close()

	_, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "published"),
		ArtifactDigest: "sha256:wrong",
		DestinationDir: filepath.Join(t.TempDir(), "current"),
		ArtifactFamily: "gguf-v1",
	}, auth)
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected digest mismatch error, got %v", err)
	}
}

func TestMaterializerReplacesStaleDestination(t *testing.T) {
	t.Parallel()

	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		layerTar: tarBytes(t, map[string]string{
			"model/config.json": `{"model_type":"phi"}`,
		}),
	})
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(filepath.Join(destination, "stale"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(destination, "stale", "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(destination, markerFileName), []byte(`{"digest":"sha256:stale","modelPath":"`+destination+`/stale"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}

	result, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: destination,
		ArtifactFamily: "hf-safetensors-v1",
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(destination, "stale", "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected stale payload to be replaced, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "model", "config.json")); err != nil {
		t.Fatalf("expected fresh materialized payload: %v", err)
	}
	if _, err := os.Stat(destination + ".previous"); !os.IsNotExist(err) {
		t.Fatalf("expected backup dir to be removed, stat err = %v", err)
	}
	if got, want := result.Digest, "sha256:deadbeef"; got != want {
		t.Fatalf("unexpected digest %q", got)
	}
}

func TestMaterializerNormalizesRootPayloadIntoStableModelPath(t *testing.T) {
	t.Parallel()

	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		layerTar: tarBytes(t, map[string]string{
			"config.json":           `{"model_type":"phi"}`,
			"tokenizer_config.json": `{"tokenizer_class":"AutoTokenizer"}`,
		}),
	})
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "current")
	result, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: destination,
		ArtifactFamily: "hf-safetensors-v1",
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	if got, want := result.ModelPath, filepath.Join(destination, modelpackports.MaterializedModelPathName); got != want {
		t.Fatalf("unexpected model path %q", got)
	}
	if _, err := os.Stat(filepath.Join(result.ModelPath, "config.json")); err != nil {
		t.Fatalf("expected normalized model config.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("expected root payload to move under stable model path, stat err = %v", err)
	}
}
