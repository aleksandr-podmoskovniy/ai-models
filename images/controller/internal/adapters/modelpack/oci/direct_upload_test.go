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
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestAdapterPublishUploadsLayerWithoutRegistryPatch(t *testing.T) {
	t.Parallel()

	registry, auth := newWritableRegistryServer(t)
	defer registry.Close()
	directUpload := newDirectUploadTestServer(t, registry, directUploadTestOptions{partSizeBytes: 64})
	defer directUpload.Close()

	modelDir := t.TempDir()
	writeTestModelFiles(t, modelDir, strings.Repeat("a", 128), "GGUF"+strings.Repeat("b", 256))

	adapter := New()
	reference := serverReference(registry.server, "published")
	publishResult, err := adapter.Publish(context.Background(), modelpackports.PublishInput{
		ModelDir:             modelDir,
		ArtifactURI:          reference,
		DirectUploadEndpoint: directUpload.server.URL,
		DirectUploadCAFile:   directUpload.caFile,
		DirectUploadInsecure: false,
	}, auth)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if got := registry.patchCount(); got != 0 {
		t.Fatalf("registry patchCount() = %d, want 0 in backing-storage-direct mode", got)
	}
	if got, want := directUpload.completeCalls, 1; got != want {
		t.Fatalf("direct upload completeCalls = %d, want %d", got, want)
	}
	if got := directUpload.uploadCalls; got == 0 {
		t.Fatal("expected direct upload PUT calls for heavy layer")
	}
	if got := directUpload.abortCalls; got != 0 {
		t.Fatalf("direct upload abortCalls = %d, want 0", got)
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

	assertFileContent(t, filepath.Join(materialized.ModelPath, "config.json"), strings.Repeat("a", 128))
	assertFileContent(t, filepath.Join(materialized.ModelPath, "weights.gguf"), "GGUF"+strings.Repeat("b", 256))
}

func TestAdapterPublishRecoversFromInterruptedDirectPartUpload(t *testing.T) {
	t.Parallel()

	registry, auth := newWritableRegistryServer(t)
	defer registry.Close()
	directUpload := newDirectUploadTestServer(t, registry, directUploadTestOptions{
		partSizeBytes: 64,
		failFirstPart: 2,
	})
	defer directUpload.Close()

	modelDir := t.TempDir()
	writeTestModelFiles(t, modelDir, strings.Repeat("c", 256), "GGUF"+strings.Repeat("d", 512))

	adapter := New()
	reference := serverReference(registry.server, "published")
	if _, err := adapter.Publish(context.Background(), modelpackports.PublishInput{
		ModelDir:             modelDir,
		ArtifactURI:          reference,
		DirectUploadEndpoint: directUpload.server.URL,
		DirectUploadCAFile:   directUpload.caFile,
		DirectUploadInsecure: false,
	}, auth); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if got := registry.patchCount(); got != 0 {
		t.Fatalf("registry patchCount() = %d, want 0 in backing-storage-direct mode", got)
	}
	if got := directUpload.listPartsCalls; got == 0 {
		t.Fatal("expected direct upload transport to recover via listParts()")
	}
	if got := directUpload.partAttemptCount(2); got < 2 {
		t.Fatalf("part 2 attempt count = %d, want retry after interrupted upload", got)
	}
	if got := directUpload.abortCalls; got != 0 {
		t.Fatalf("direct upload abortCalls = %d, want 0", got)
	}
}

func writeTestModelFiles(t *testing.T, modelDir, configJSON, ggufPayload string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(modelDir, "config.json"), []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "weights.gguf"), []byte(ggufPayload), 0o644); err != nil {
		t.Fatalf("WriteFile(weights.gguf) error = %v", err)
	}
}
