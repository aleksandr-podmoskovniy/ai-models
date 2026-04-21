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
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestAdapterPublishAndMaterializeObjectSourceLayer(t *testing.T) {
	t.Parallel()

	server, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{})

	adapter := New()
	reference := serverReference(server.server, "object-source")
	publishResult, err := adapter.Publish(context.Background(), withDirectUploadInput(modelpackports.PublishInput{
		ArtifactURI: reference,
		Layers: []modelpackports.PublishLayer{
			{
				SourcePath: "s3://artifacts/raw/1111-2222/source-url/.mirror/huggingface/owner/model/deadbeef",
				TargetPath: "model",
				Base:       modelpackports.LayerBaseModel,
				Format:     modelpackports.LayerFormatTar,
				ObjectSource: &modelpackports.PublishObjectSource{
					Reader: &fakeObjectReader{
						files: map[string]fakeObjectFile{
							"mirror/config.json": {
								payload: []byte("{\"arch\":\"tiny\"}\n"),
								etag:    `"etag-config"`,
							},
							"mirror/model.safetensors": {
								payload: []byte("weights"),
								etag:    `"etag-model"`,
							},
						},
					},
					Files: []modelpackports.PublishObjectFile{
						{SourcePath: "mirror/config.json", TargetPath: "config.json", SizeBytes: int64(len("{\"arch\":\"tiny\"}\n")), ETag: `"etag-config"`},
					},
				},
			},
			{
				SourcePath: "s3://artifacts/raw/1111-2222/source-url/.mirror/huggingface/owner/model/deadbeef",
				TargetPath: "model/model.safetensors",
				Base:       modelpackports.LayerBaseModel,
				Format:     modelpackports.LayerFormatRaw,
				ObjectSource: &modelpackports.PublishObjectSource{
					Reader: &fakeObjectReader{
						files: map[string]fakeObjectFile{
							"mirror/config.json": {
								payload: []byte("{\"arch\":\"tiny\"}\n"),
								etag:    `"etag-config"`,
							},
							"mirror/model.safetensors": {
								payload: []byte("weights"),
								etag:    `"etag-model"`,
							},
						},
					},
					Files: []modelpackports.PublishObjectFile{
						{SourcePath: "mirror/model.safetensors", TargetPath: "model/model.safetensors", SizeBytes: int64(len("weights")), ETag: `"etag-model"`},
					},
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
}

type fakeObjectReader struct {
	files         map[string]fakeObjectFile
	openReadCalls int
	rangeCalls    int
}

func (f *fakeObjectReader) OpenRead(_ context.Context, sourcePath string) (modelpackports.OpenReadResult, error) {
	file, found := f.files[sourcePath]
	if !found {
		return modelpackports.OpenReadResult{}, io.EOF
	}
	f.openReadCalls++
	return modelpackports.OpenReadResult{
		Body:      io.NopCloser(bytes.NewReader(file.payload)),
		SizeBytes: int64(len(file.payload)),
		ETag:      file.etag,
	}, nil
}

func (f *fakeObjectReader) OpenReadRange(_ context.Context, sourcePath string, offset, length int64) (modelpackports.OpenReadResult, error) {
	file, found := f.files[sourcePath]
	if !found {
		return modelpackports.OpenReadResult{}, io.EOF
	}
	f.rangeCalls++
	if offset < 0 || offset > int64(len(file.payload)) {
		return modelpackports.OpenReadResult{}, io.EOF
	}
	payload := file.payload[offset:]
	if length >= 0 && length < int64(len(payload)) {
		payload = payload[:length]
	}
	return modelpackports.OpenReadResult{
		Body:      io.NopCloser(bytes.NewReader(payload)),
		SizeBytes: int64(len(payload)),
		ETag:      file.etag,
	}, nil
}

type fakeObjectFile struct {
	payload []byte
	etag    string
}

func TestAdapterPublishObjectSourceUsesRangeReadsOnInterruptedUpload(t *testing.T) {
	t.Parallel()

	server, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{
		partSizeBytes: 64,
		failFirstPart: 2,
	})

	reader := &fakeObjectReader{
		files: map[string]fakeObjectFile{
			"mirror/config.json": {
				payload: []byte("{\"arch\":\"tiny\"}\n"),
				etag:    `"etag-config"`,
			},
			"mirror/model.safetensors": {
				payload: []byte(strings.Repeat("weights", 64)),
				etag:    `"etag-model"`,
			},
		},
	}

	adapter := New()
	reference := serverReference(server.server, "object-source-ranged")
	if _, err := adapter.Publish(context.Background(), withDirectUploadInput(modelpackports.PublishInput{
		ArtifactURI: reference,
		Layers: []modelpackports.PublishLayer{
			{
				SourcePath: "https://huggingface.co/owner/model?revision=deadbeef",
				TargetPath: "model.safetensors",
				Base:       modelpackports.LayerBaseModel,
				Format:     modelpackports.LayerFormatRaw,
				ObjectSource: &modelpackports.PublishObjectSource{
					Reader: reader,
					Files: []modelpackports.PublishObjectFile{
						{SourcePath: "mirror/model.safetensors", TargetPath: "model.safetensors", SizeBytes: int64(len(strings.Repeat("weights", 64))), ETag: `"etag-model"`},
					},
				},
			},
		},
	}, directUpload), auth); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if got := reader.rangeCalls; got == 0 {
		t.Fatal("expected ranged object reads after interrupted upload")
	}
	if got := reader.openReadCalls; got != 0 {
		t.Fatalf("OpenRead() calls = %d, want 0 on raw ranged late-digest path", got)
	}
}
