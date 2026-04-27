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

package publishworker

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestBuildHuggingFacePublishLayersUsesSourceMirrorObjectSource(t *testing.T) {
	t.Parallel()

	staging := &fakeUploadStaging{
		objects: map[string][]byte{
			"raw/1111-2222/source-url/.mirror/huggingface/owner/model/deadbeef/files/config.json":       []byte(`{"arch":"tiny"}`),
			"raw/1111-2222/source-url/.mirror/huggingface/owner/model/deadbeef/files/model.safetensors": []byte("weights"),
		},
	}
	remote := sourcefetch.RemoteResult{
		SelectedFiles: []string{"config.json", "model.safetensors"},
		SourceMirror: &sourcefetch.SourceMirrorSnapshot{
			CleanupPrefix: "raw/1111-2222/source-url/.mirror/huggingface/owner/model/deadbeef",
		},
	}

	layers, err := buildHuggingFacePublishLayers(context.Background(), Options{
		RawStageBucket: "artifacts",
		UploadStaging:  staging,
	}, remote)
	if err != nil {
		t.Fatalf("buildHuggingFacePublishLayers() error = %v", err)
	}
	if got, want := len(layers), 2; got != want {
		t.Fatalf("unexpected layer count %d", got)
	}
	if layers[0].ObjectSource == nil || layers[1].ObjectSource == nil {
		t.Fatal("expected object source layers")
	}
	if got, want := layers[0].Base, modelpackports.LayerBaseModel; got != want {
		t.Fatalf("unexpected first layer base %q", got)
	}
	if got, want := layers[0].Format, modelpackports.LayerFormatTar; got != want {
		t.Fatalf("unexpected first layer format %q", got)
	}
	if got, want := layers[0].TargetPath, modelpackports.MaterializedModelPathName; got != want {
		t.Fatalf("unexpected first target path %q", got)
	}
	if got, want := len(layers[0].ObjectSource.Files), 1; got != want {
		t.Fatalf("unexpected first object source file count %d", got)
	}
	if got, want := layers[0].ObjectSource.Files[0].TargetPath, "config.json"; got != want {
		t.Fatalf("unexpected bundled target path %q", got)
	}
	if got, want := layers[1].Format, modelpackports.LayerFormatRaw; got != want {
		t.Fatalf("unexpected second layer format %q", got)
	}
	if got, want := layers[1].TargetPath, "model/model.safetensors"; got != want {
		t.Fatalf("unexpected second target path %q", got)
	}
	if got, want := layers[1].ObjectSource.Files[0].SizeBytes, int64(len("weights")); got != want {
		t.Fatalf("unexpected second file size %d", got)
	}
}

func TestBuildHuggingFacePublishLayersUsesDirectRemoteObjectSource(t *testing.T) {
	t.Parallel()

	remote := sourcefetch.RemoteResult{
		Provenance: sourcefetch.RemoteProvenance{
			ExternalReference: "owner/model",
			ResolvedRevision:  "deadbeef",
		},
		ObjectSource: &sourcefetch.RemoteObjectSource{
			Reader: fakeRemoteReadSource{},
			Files: []sourcefetch.RemoteObjectFile{
				{
					SourcePath: "https://huggingface.example/owner/model/resolve/deadbeef/config.json",
					TargetPath: "config.json",
					SizeBytes:  17,
					ETag:       `"etag-config"`,
				},
				{
					SourcePath: "https://huggingface.example/owner/model/resolve/deadbeef/model.safetensors",
					TargetPath: "model.safetensors",
					SizeBytes:  23,
					ETag:       `"etag-model"`,
				},
			},
		},
	}

	layers, err := buildHuggingFacePublishLayers(context.Background(), Options{}, remote)
	if err != nil {
		t.Fatalf("buildHuggingFacePublishLayers() error = %v", err)
	}
	if got, want := len(layers), 2; got != want {
		t.Fatalf("unexpected layer count %d", got)
	}
	if layers[0].ObjectSource == nil || layers[1].ObjectSource == nil {
		t.Fatal("expected object source layers")
	}
	if got, want := layers[0].SourcePath, "https://huggingface.co/owner/model?revision=deadbeef"; got != want {
		t.Fatalf("unexpected first source path %q", got)
	}
	if got, want := layers[0].Format, modelpackports.LayerFormatTar; got != want {
		t.Fatalf("unexpected first layer format %q", got)
	}
	if got, want := layers[0].TargetPath, modelpackports.MaterializedModelPathName; got != want {
		t.Fatalf("unexpected first target path %q", got)
	}
	if got, want := layers[0].ObjectSource.Files[0].ETag, `"etag-config"`; got != want {
		t.Fatalf("unexpected first file etag %q", got)
	}
	if got, want := layers[0].ObjectSource.Files[0].TargetPath, "config.json"; got != want {
		t.Fatalf("unexpected bundled target path %q", got)
	}
	if got, want := layers[1].Format, modelpackports.LayerFormatRaw; got != want {
		t.Fatalf("unexpected second layer format %q", got)
	}
	if got, want := layers[1].TargetPath, "model/model.safetensors"; got != want {
		t.Fatalf("unexpected second target path %q", got)
	}
}

type fakeRemoteReadSource struct{}

func (fakeRemoteReadSource) OpenRead(context.Context, string) (sourcefetch.RemoteOpenReadResult, error) {
	return sourcefetch.RemoteOpenReadResult{
		Body:      io.NopCloser(bytes.NewReader(nil)),
		SizeBytes: 0,
		ETag:      "",
	}, nil
}
