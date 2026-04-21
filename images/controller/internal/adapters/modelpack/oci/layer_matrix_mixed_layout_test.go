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
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestAdapterPublishAndMaterializeMixedInternalLayersKeepStableModelContract(t *testing.T) {
	t.Parallel()

	server, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{})

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte("{\"arch\":\"tiny\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	weightsDir := filepath.Join(t.TempDir(), "weights")
	if err := os.MkdirAll(weightsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(weights) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(weightsDir, "model.safetensors"), []byte("weights"), 0o644); err != nil {
		t.Fatalf("WriteFile(weights) error = %v", err)
	}

	adapter := New()
	reference := serverReference(server.server, "mixed-layout")
	publishResult, err := adapter.Publish(context.Background(), withDirectUploadInput(modelpackports.PublishInput{
		ArtifactURI: reference,
		Layers: []modelpackports.PublishLayer{
			{
				SourcePath:  configPath,
				TargetPath:  "model/config.json",
				Base:        modelpackports.LayerBaseModelConfig,
				Format:      modelpackports.LayerFormatRaw,
				Compression: modelpackports.LayerCompressionNone,
			},
			{
				SourcePath:  weightsDir,
				TargetPath:  "model",
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatTar,
				Compression: modelpackports.LayerCompressionNone,
			},
		},
	}, directUpload), auth)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	inspectPayload, err := InspectRemote(context.Background(), reference, auth)
	if err != nil {
		t.Fatalf("InspectRemote() error = %v", err)
	}
	configBlob, _ := inspectPayload["configBlob"].(map[string]any)
	descriptor, _ := configBlob["descriptor"].(map[string]any)
	if got, want := descriptor["name"], "model"; got != want {
		t.Fatalf("config descriptor name = %#v, want %q", got, want)
	}

	destination := filepath.Join(t.TempDir(), "materialized")
	materialized, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    reference,
		ArtifactDigest: publishResult.Digest,
		DestinationDir: destination,
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	if got, want := materialized.ModelPath, filepath.Join(destination, modelpackports.MaterializedModelPathName); got != want {
		t.Fatalf("materialized model path = %q, want stable contract path %q", got, want)
	}
	assertFileContent(t, filepath.Join(materialized.ModelPath, "config.json"), "{\"arch\":\"tiny\"}\n")
	assertFileContent(t, filepath.Join(materialized.ModelPath, "model.safetensors"), "weights")
}
