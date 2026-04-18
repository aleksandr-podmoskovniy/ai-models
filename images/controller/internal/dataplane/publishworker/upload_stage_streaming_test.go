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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func TestPublishFromUploadStageStreamsDirectGGUFWithoutDownload(t *testing.T) {
	t.Parallel()

	payload := []byte("GGUFweights")
	staging := &fakeUploadStageStreamingReader{
		payload: payload,
	}
	publisher := fakePublisher{
		onPublish: func(input modelpackports.PublishInput) error {
			if got, want := input.ModelDir, "s3://artifacts/uploads/model.bin"; got != want {
				t.Fatalf("unexpected ModelDir %q", got)
			}
			if got, want := len(input.Layers), 1; got != want {
				t.Fatalf("unexpected layer count %d", got)
			}
			layer := input.Layers[0]
			if layer.ObjectSource == nil {
				t.Fatal("expected object source layer")
			}
			if got, want := layer.SourcePath, "s3://artifacts/uploads/model.bin"; got != want {
				t.Fatalf("unexpected layer source path %q", got)
			}
			if got, want := len(layer.ObjectSource.Files), 1; got != want {
				t.Fatalf("unexpected object source file count %d", got)
			}
			file := layer.ObjectSource.Files[0]
			if got, want := file.SourcePath, "uploads/model.bin"; got != want {
				t.Fatalf("unexpected object source path %q", got)
			}
			if got, want := file.TargetPath, "model.bin"; got != want {
				t.Fatalf("unexpected target path %q", got)
			}
			if got, want := file.SizeBytes, int64(len(payload)); got != want {
				t.Fatalf("unexpected file size %d", got)
			}
			return nil
		},
	}

	result, err := run(context.Background(), Options{
		SourceType:  modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI: "registry.example.com/ai-models/catalog/model:published",
		UploadStage: &cleanuphandle.UploadStagingHandle{
			Bucket:   "artifacts",
			Key:      "uploads/model.bin",
			FileName: "model.bin",
		},
		Task:               "text-generation",
		UploadStaging:      staging,
		ModelPackPublisher: publisher,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := result.Resolved.Format, "GGUF"; got != want {
		t.Fatalf("unexpected resolved format %q", got)
	}
	if got, want := staging.downloadCalls, 0; got != want {
		t.Fatalf("unexpected download call count %d", got)
	}
	if got := staging.rangeCalls; got == 0 {
		t.Fatal("expected ranged validation read")
	}
	if got, want := staging.deleteCalls, 1; got != want {
		t.Fatalf("unexpected delete call count %d", got)
	}
}

func TestPublishFromUploadStageRejectsInvalidDirectSafetensorsWithoutDownload(t *testing.T) {
	t.Parallel()

	staging := &fakeUploadStageStreamingReader{
		payload: []byte("plain"),
	}
	publisher := fakePublisher{
		onPublish: func(modelpackports.PublishInput) error {
			t.Fatal("publisher must not be called for invalid staged direct safetensors upload")
			return nil
		},
	}

	_, err := run(context.Background(), Options{
		SourceType:  modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI: "registry.example.com/ai-models/catalog/model:published",
		UploadStage: &cleanuphandle.UploadStagingHandle{
			Bucket:   "artifacts",
			Key:      "uploads/model.safetensors",
			FileName: "model.safetensors",
		},
		UploadStaging:      staging,
		ModelPackPublisher: publisher,
	})
	if err == nil {
		t.Fatal("expected invalid staged direct safetensors upload to fail")
	}
	if got, want := staging.downloadCalls, 0; got != want {
		t.Fatalf("unexpected download call count %d", got)
	}
	if got, want := staging.deleteCalls, 0; got != want {
		t.Fatalf("unexpected delete call count %d", got)
	}
}

type fakeUploadStageStreamingReader struct {
	payload       []byte
	downloadCalls int
	rangeCalls    int
	deleteCalls   int
}

func (f *fakeUploadStageStreamingReader) StartMultipartUpload(context.Context, uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	return uploadstagingports.StartMultipartUploadOutput{}, nil
}

func (f *fakeUploadStageStreamingReader) PresignUploadPart(context.Context, uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{}, nil
}

func (f *fakeUploadStageStreamingReader) ListMultipartUploadParts(context.Context, uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	return nil, nil
}

func (f *fakeUploadStageStreamingReader) CompleteMultipartUpload(context.Context, uploadstagingports.CompleteMultipartUploadInput) error {
	return nil
}

func (f *fakeUploadStageStreamingReader) AbortMultipartUpload(context.Context, uploadstagingports.AbortMultipartUploadInput) error {
	return nil
}

func (f *fakeUploadStageStreamingReader) Stat(context.Context, uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	return uploadstagingports.ObjectStat{
		SizeBytes: int64(len(f.payload)),
		ETag:      `"stage-etag"`,
	}, nil
}

func (f *fakeUploadStageStreamingReader) Download(context.Context, uploadstagingports.DownloadInput) error {
	f.downloadCalls++
	return nil
}

func (f *fakeUploadStageStreamingReader) OpenRead(context.Context, uploadstagingports.OpenReadInput) (uploadstagingports.OpenReadOutput, error) {
	return uploadstagingports.OpenReadOutput{
		Body:      io.NopCloser(bytes.NewReader(f.payload)),
		SizeBytes: int64(len(f.payload)),
		ETag:      `"stage-etag"`,
	}, nil
}

func (f *fakeUploadStageStreamingReader) OpenReadRange(_ context.Context, input uploadstagingports.OpenReadRangeInput) (uploadstagingports.OpenReadOutput, error) {
	f.rangeCalls++
	start := input.Offset
	if start < 0 {
		start = 0
	}
	end := int64(len(f.payload))
	if input.Length >= 0 && start+input.Length < end {
		end = start + input.Length
	}
	if start > int64(len(f.payload)) {
		start = int64(len(f.payload))
	}
	if end < start {
		end = start
	}
	return uploadstagingports.OpenReadOutput{
		Body:      io.NopCloser(bytes.NewReader(f.payload[start:end])),
		SizeBytes: end - start,
		ETag:      `"stage-etag"`,
	}, nil
}

func (f *fakeUploadStageStreamingReader) Upload(context.Context, uploadstagingports.UploadInput) error {
	return nil
}

func (f *fakeUploadStageStreamingReader) Delete(context.Context, uploadstagingports.DeleteInput) error {
	f.deleteCalls++
	return nil
}

func (*fakeUploadStageStreamingReader) DeletePrefix(context.Context, uploadstagingports.DeletePrefixInput) error {
	return nil
}
