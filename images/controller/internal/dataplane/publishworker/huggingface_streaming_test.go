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
	"os"
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func TestBuildHuggingFacePublishLayersUsesSourceMirrorObjectSource(t *testing.T) {
	t.Parallel()

	staging := &fakeMirrorReadStaging{
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
	if got, want := len(layers), 1; got != want {
		t.Fatalf("unexpected layer count %d", got)
	}
	layer := layers[0]
	if layer.ObjectSource == nil {
		t.Fatal("expected object source layer")
	}
	if got, want := layer.Base, modelpackports.LayerBaseModel; got != want {
		t.Fatalf("unexpected layer base %q", got)
	}
	if got, want := layer.TargetPath, modelpackports.MaterializedModelPathName; got != want {
		t.Fatalf("unexpected target path %q", got)
	}
	if got, want := len(layer.ObjectSource.Files), 2; got != want {
		t.Fatalf("unexpected object source file count %d", got)
	}
	if got, want := layer.ObjectSource.Files[0].TargetPath, "config.json"; got != want {
		t.Fatalf("unexpected first target path %q", got)
	}
	if got, want := layer.ObjectSource.Files[1].SizeBytes, int64(len("weights")); got != want {
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
	if got, want := len(layers), 1; got != want {
		t.Fatalf("unexpected layer count %d", got)
	}
	layer := layers[0]
	if layer.ObjectSource == nil {
		t.Fatal("expected object source layer")
	}
	if got, want := layer.SourcePath, "https://huggingface.co/owner/model?revision=deadbeef"; got != want {
		t.Fatalf("unexpected source path %q", got)
	}
	if got, want := len(layer.ObjectSource.Files), 2; got != want {
		t.Fatalf("unexpected object source file count %d", got)
	}
	if got, want := layer.ObjectSource.Files[0].ETag, `"etag-config"`; got != want {
		t.Fatalf("unexpected first file etag %q", got)
	}
	if got, want := layer.ObjectSource.Files[1].TargetPath, "model.safetensors"; got != want {
		t.Fatalf("unexpected second target path %q", got)
	}
}

type fakeMirrorReadStaging struct {
	objects map[string][]byte
}

func (f *fakeMirrorReadStaging) StartMultipartUpload(context.Context, uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	return uploadstagingports.StartMultipartUploadOutput{}, nil
}

func (f *fakeMirrorReadStaging) PresignUploadPart(context.Context, uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{}, nil
}

func (f *fakeMirrorReadStaging) ListMultipartUploadParts(context.Context, uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	return nil, nil
}

func (f *fakeMirrorReadStaging) CompleteMultipartUpload(context.Context, uploadstagingports.CompleteMultipartUploadInput) error {
	return nil
}

func (f *fakeMirrorReadStaging) AbortMultipartUpload(context.Context, uploadstagingports.AbortMultipartUploadInput) error {
	return nil
}

func (f *fakeMirrorReadStaging) Stat(_ context.Context, input uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	payload, found := f.objects[input.Key]
	if !found {
		return uploadstagingports.ObjectStat{}, os.ErrNotExist
	}
	return uploadstagingports.ObjectStat{SizeBytes: int64(len(payload)), ETag: `"etag-complete"`}, nil
}

func (f *fakeMirrorReadStaging) Download(context.Context, uploadstagingports.DownloadInput) error {
	return nil
}

func (f *fakeMirrorReadStaging) Upload(context.Context, uploadstagingports.UploadInput) error {
	return nil
}

func (f *fakeMirrorReadStaging) Delete(context.Context, uploadstagingports.DeleteInput) error {
	return nil
}

func (f *fakeMirrorReadStaging) DeletePrefix(_ context.Context, input uploadstagingports.DeletePrefixInput) error {
	for key := range f.objects {
		if strings.HasPrefix(key, input.Prefix) {
			delete(f.objects, key)
		}
	}
	return nil
}

func (f *fakeMirrorReadStaging) OpenRead(_ context.Context, input uploadstagingports.OpenReadInput) (uploadstagingports.OpenReadOutput, error) {
	payload, found := f.objects[input.Key]
	if !found {
		return uploadstagingports.OpenReadOutput{}, os.ErrNotExist
	}
	return uploadstagingports.OpenReadOutput{
		Body:      io.NopCloser(bytes.NewReader(payload)),
		SizeBytes: int64(len(payload)),
		ETag:      `"etag-complete"`,
	}, nil
}

type fakeRemoteReadSource struct{}

func (fakeRemoteReadSource) OpenRead(context.Context, string) (sourcefetch.RemoteOpenReadResult, error) {
	return sourcefetch.RemoteOpenReadResult{
		Body:      io.NopCloser(bytes.NewReader(nil)),
		SizeBytes: 0,
		ETag:      "",
	}, nil
}
