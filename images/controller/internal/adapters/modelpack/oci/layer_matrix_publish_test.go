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

func TestAdapterPublishAndMaterializeFullLayerMatrix(t *testing.T) {
	t.Parallel()

	server, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{})

	modelDir := filepath.Join(t.TempDir(), "model")
	codeDir := filepath.Join(t.TempDir(), "code")
	datasetDir := filepath.Join(t.TempDir(), "datasets")
	docFile := filepath.Join(t.TempDir(), "README.md")
	modelConfig := filepath.Join(t.TempDir(), "config.json")
	for _, dir := range []string{modelDir, codeDir, datasetDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(modelDir, "weights.gguf"), []byte("GGUF-full-matrix"), 0o644); err != nil {
		t.Fatalf("WriteFile(weights.gguf) error = %v", err)
	}
	if err := os.WriteFile(modelConfig, []byte("{\"arch\":\"tiny\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(codeDir, "serve.py"), []byte("print('serve')\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(serve.py) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(datasetDir, "train.jsonl"), []byte("{\"input\":\"hi\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(train.jsonl) error = %v", err)
	}
	if err := os.WriteFile(docFile, []byte("# Model\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}

	adapter := New()
	reference := serverReference(server.server, "published")
	publishResult, err := adapter.Publish(context.Background(), withDirectUploadInput(modelpackports.PublishInput{
		ArtifactURI: reference,
		Layers: []modelpackports.PublishLayer{
			{
				SourcePath:  modelDir,
				TargetPath:  "model",
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatTar,
				Compression: modelpackports.LayerCompressionNone,
			},
			{
				SourcePath: modelConfig,
				TargetPath: "model/config.json",
				Base:       modelpackports.LayerBaseModelConfig,
				Format:     modelpackports.LayerFormatRaw,
			},
			{
				SourcePath:  codeDir,
				TargetPath:  "code",
				Base:        modelpackports.LayerBaseCode,
				Format:      modelpackports.LayerFormatTar,
				Compression: modelpackports.LayerCompressionGzip,
			},
			{
				SourcePath:  datasetDir,
				TargetPath:  "datasets",
				Base:        modelpackports.LayerBaseDataset,
				Format:      modelpackports.LayerFormatTar,
				Compression: modelpackports.LayerCompressionZstd,
			},
			{
				SourcePath: docFile,
				TargetPath: "docs/README.md",
				Base:       modelpackports.LayerBaseDoc,
				Format:     modelpackports.LayerFormatRaw,
			},
		},
	}, directUpload), auth)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if publishResult.Digest == "" {
		t.Fatal("Publish() must return artifact digest")
	}

	materialized, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    reference,
		ArtifactDigest: publishResult.Digest,
		DestinationDir: filepath.Join(t.TempDir(), "materialized"),
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	assertFileContent(t, filepath.Join(materialized.ModelPath, "weights.gguf"), "GGUF-full-matrix")
	assertFileContent(t, filepath.Join(materialized.ModelPath, "config.json"), "{\"arch\":\"tiny\"}\n")
	assertFileContent(t, filepath.Join(filepath.Dir(materialized.ModelPath), "code", "serve.py"), "print('serve')\n")
	assertFileContent(t, filepath.Join(filepath.Dir(materialized.ModelPath), "datasets", "train.jsonl"), "{\"input\":\"hi\"}\n")
	assertFileContent(t, filepath.Join(filepath.Dir(materialized.ModelPath), "docs", "README.md"), "# Model\n")
}

func TestAdapterPublishAndMaterializeArchiveSourceLayer(t *testing.T) {
	t.Parallel()

	server, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{})

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tgz")
	if err := writeTestGzipTar(
		archivePath,
		map[string]string{
			"checkpoint/config.json":       "{\"arch\":\"tiny\"}\n",
			"checkpoint/model.safetensors": "weights",
			"checkpoint/model.py":          "print('helper')\n",
		},
	); err != nil {
		t.Fatalf("writeTestGzipTar() error = %v", err)
	}

	adapter := New()
	reference := serverReference(server.server, "archive-source")
	publishResult, err := adapter.Publish(context.Background(), withDirectUploadInput(modelpackports.PublishInput{
		ArtifactURI: reference,
		Layers: []modelpackports.PublishLayer{
			{
				SourcePath:  archivePath,
				TargetPath:  "model",
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatTar,
				Compression: modelpackports.LayerCompressionGzip,
				Archive: &modelpackports.PublishArchiveSource{
					StripPathPrefix: "checkpoint",
					SelectedFiles:   []string{"config.json", "model.safetensors"},
				},
			},
		},
	}, directUpload), auth)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	materialized, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    reference,
		ArtifactDigest: publishResult.Digest,
		DestinationDir: filepath.Join(t.TempDir(), "materialized"),
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	assertFileContent(t, filepath.Join(materialized.ModelPath, "config.json"), "{\"arch\":\"tiny\"}\n")
	assertFileContent(t, filepath.Join(materialized.ModelPath, "model.safetensors"), "weights")
	if _, err := os.Stat(filepath.Join(materialized.ModelPath, "model.py")); !os.IsNotExist(err) {
		t.Fatalf("expected helper script to be stripped, got err=%v", err)
	}
}
