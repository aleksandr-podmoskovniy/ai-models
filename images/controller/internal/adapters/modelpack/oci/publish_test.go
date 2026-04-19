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

func TestAdapterPublishMaterializeAndRemove(t *testing.T) {
	t.Parallel()

	server, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{})

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
	publishResult, err := adapter.Publish(context.Background(), withDirectUploadInput(modelpackports.PublishInput{
		ModelDir:    modelDir,
		ArtifactURI: reference,
	}, directUpload), auth)
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
	if got := server.patchCount(); got != 0 {
		t.Fatalf("Publish() patchCount() = %d, want 0 on direct upload path", got)
	}
	if got := directUpload.uploadCalls; got == 0 {
		t.Fatal("expected direct upload PUT calls for heavy layer")
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

func TestAdapterPublishRecoversFromInterruptedChunkedUpload(t *testing.T) {
	t.Parallel()

	server, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{
		partSizeBytes: 64,
		failFirstPart: 2,
	})

	modelDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modelDir, "config.json"), []byte(strings.Repeat("a", 128)), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "weights.gguf"), []byte("GGUF"+strings.Repeat("b", 256)), 0o644); err != nil {
		t.Fatalf("WriteFile(weights.gguf) error = %v", err)
	}

	adapter := New()
	reference := serverReference(server.server, "published")
	if _, err := adapter.Publish(context.Background(), withDirectUploadInput(modelpackports.PublishInput{
		ModelDir:    modelDir,
		ArtifactURI: reference,
	}, directUpload), auth); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if got := server.patchCount(); got != 0 {
		t.Fatalf("Publish() patchCount() = %d, want 0 on direct upload path", got)
	}
	if got := directUpload.listPartsCalls; got == 0 {
		t.Fatal("expected direct upload recovery via listParts()")
	}
	if got := directUpload.partAttemptCount(2); got < 2 {
		t.Fatalf("part 2 attempt count = %d, want retry after interrupted direct upload", got)
	}
}
