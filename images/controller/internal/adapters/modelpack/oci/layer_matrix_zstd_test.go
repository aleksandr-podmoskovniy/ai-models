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

func TestAdapterPublishAndMaterializeZstdArchiveSourceLayer(t *testing.T) {
	t.Parallel()

	server, auth := newWritableRegistryServer(t)
	defer server.Close()

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tar.zst")
	if err := writeTestZstdTar(
		archivePath,
		map[string]string{
			"checkpoint/config.json":       "{\"arch\":\"tiny\"}\n",
			"checkpoint/model.safetensors": "weights",
			"checkpoint/model.py":          "print('helper')\n",
		},
	); err != nil {
		t.Fatalf("writeTestZstdTar() error = %v", err)
	}

	adapter := New()
	reference := serverReference(server.server, "zstd-archive-source")
	publishResult, err := adapter.Publish(context.Background(), modelpackports.PublishInput{
		ArtifactURI: reference,
		Layers: []modelpackports.PublishLayer{
			{
				SourcePath:  archivePath,
				TargetPath:  "model",
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatTar,
				Compression: modelpackports.LayerCompressionZstd,
				Archive: &modelpackports.PublishArchiveSource{
					StripPathPrefix: "checkpoint",
					SelectedFiles:   []string{"config.json", "model.safetensors"},
				},
			},
		},
	}, auth)
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

func TestAdapterPublishAndMaterializeZstdArchiveSourceReaderLayer(t *testing.T) {
	t.Parallel()

	server, auth := newWritableRegistryServer(t)
	defer server.Close()

	archivePath := filepath.Join(t.TempDir(), "checkpoint.tar.zst")
	if err := writeTestZstdTar(
		archivePath,
		map[string]string{
			"checkpoint/config.json":       "{\"arch\":\"tiny\"}\n",
			"checkpoint/model.safetensors": "weights",
			"checkpoint/model.py":          "print('helper')\n",
		},
	); err != nil {
		t.Fatalf("writeTestZstdTar() error = %v", err)
	}
	payload, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	adapter := New()
	reference := serverReference(server.server, "zstd-archive-source-reader")
	publishResult, err := adapter.Publish(context.Background(), modelpackports.PublishInput{
		ArtifactURI: reference,
		Layers: []modelpackports.PublishLayer{
			{
				SourcePath:  "uploads/checkpoint.tar.zst",
				TargetPath:  "model",
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatTar,
				Compression: modelpackports.LayerCompressionZstd,
				Archive: &modelpackports.PublishArchiveSource{
					StripPathPrefix: "checkpoint",
					SelectedFiles:   []string{"config.json", "model.safetensors"},
					Reader:          fakeArchiveLayerReader{payload: payload},
				},
			},
		},
	}, auth)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	materialized, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    reference,
		ArtifactDigest: publishResult.Digest,
		DestinationDir: filepath.Join(t.TempDir(), "materialized-reader"),
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
