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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func TestPublishFromUploadStageStreamsArchiveWithoutLocalDownload(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		fileName         string
		create           func(string) error
		expectDownloaded bool
	}{
		{
			name:             "tar",
			fileName:         "model.tar",
			expectDownloaded: false,
			create: func(path string) error {
				return createTestTar(path,
					tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
					tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
				)
			},
		},
		{
			name:             "zip",
			fileName:         "model.zip",
			expectDownloaded: false,
			create: func(path string) error {
				return createTestZip(path,
					tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
					tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
				)
			},
		},
		{
			name:             "tar.zst",
			fileName:         "model.tar.zst",
			expectDownloaded: false,
			create: func(path string) error {
				return createTestZstdTar(path,
					tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
					tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
				)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			archivePath := filepath.Join(t.TempDir(), tc.fileName)
			if err := tc.create(archivePath); err != nil {
				t.Fatalf("create archive error = %v", err)
			}
			payload, err := os.ReadFile(archivePath)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", tc.fileName, err)
			}
			staging := &fakeUploadStagingForPublish{
				payload: payload,
			}
			publisher := fakePublisher{
				onPublish: func(input modelpackports.PublishInput) error {
					if tc.expectDownloaded {
						if strings.TrimSpace(staging.downloadDestination) == "" {
							return errors.New("download destination was not captured")
						}
						if _, err := os.Stat(staging.downloadDestination); err != nil {
							return err
						}
					} else if strings.TrimSpace(staging.downloadDestination) != "" {
						return errors.New("unexpected local download for staged archive streaming path")
					}
					if len(input.Layers) != 1 || input.Layers[0].Archive == nil {
						return errors.New("expected archive streaming publish layer")
					}
					if !tc.expectDownloaded && input.Layers[0].Archive.Reader == nil {
						return errors.New("expected archive reader for staged object-source path")
					}
					if tc.fileName == "model.zip" && input.Layers[0].Archive.SizeBytes <= 0 {
						return errors.New("expected positive archive size for staged zip path")
					}
					return nil
				},
			}

			result, err := run(context.Background(), Options{
				SourceType:  modelsv1alpha1.ModelSourceTypeUpload,
				ArtifactURI: "registry.example.com/ai-models/catalog/model:published",
				UploadStage: &cleanuphandle.UploadStagingHandle{
					Bucket:   "artifacts",
					Key:      "uploads/" + tc.fileName,
					FileName: tc.fileName,
				},
				Task:               "text-generation",
				UploadStaging:      staging,
				ModelPackPublisher: publisher,
			})
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if got, want := result.Resolved.Format, "Safetensors"; got != want {
				t.Fatalf("unexpected resolved format %q", got)
			}
			if got, want := staging.deleteCalls, 1; got != want {
				t.Fatalf("unexpected upload staging delete count %d", got)
			}
			if !tc.expectDownloaded {
				return
			}
			if _, err := os.Stat(staging.downloadDestination); !os.IsNotExist(err) {
				t.Fatalf("expected local staging download cleanup, got err=%v", err)
			}
		})
	}
}

type fakeUploadStagingForPublish struct {
	payload             []byte
	downloadDestination string
	deleteCalls         int
}

func (f *fakeUploadStagingForPublish) StartMultipartUpload(context.Context, uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	return uploadstagingports.StartMultipartUploadOutput{}, nil
}

func (f *fakeUploadStagingForPublish) PresignUploadPart(context.Context, uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{}, nil
}

func (f *fakeUploadStagingForPublish) ListMultipartUploadParts(context.Context, uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	return nil, nil
}

func (f *fakeUploadStagingForPublish) CompleteMultipartUpload(context.Context, uploadstagingports.CompleteMultipartUploadInput) error {
	return nil
}

func (f *fakeUploadStagingForPublish) AbortMultipartUpload(context.Context, uploadstagingports.AbortMultipartUploadInput) error {
	return nil
}

func (f *fakeUploadStagingForPublish) Stat(context.Context, uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	return uploadstagingports.ObjectStat{SizeBytes: int64(len(f.payload)), ETag: `"stage-etag"`}, nil
}

func (f *fakeUploadStagingForPublish) Download(_ context.Context, input uploadstagingports.DownloadInput) error {
	f.downloadDestination = input.DestinationPath
	return os.WriteFile(input.DestinationPath, f.payload, 0o644)
}

func (f *fakeUploadStagingForPublish) OpenRead(context.Context, uploadstagingports.OpenReadInput) (uploadstagingports.OpenReadOutput, error) {
	return uploadstagingports.OpenReadOutput{
		Body:      io.NopCloser(bytes.NewReader(f.payload)),
		SizeBytes: int64(len(f.payload)),
		ETag:      `"stage-etag"`,
	}, nil
}

func (f *fakeUploadStagingForPublish) OpenReadRange(_ context.Context, input uploadstagingports.OpenReadRangeInput) (uploadstagingports.OpenReadOutput, error) {
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

func (f *fakeUploadStagingForPublish) Upload(context.Context, uploadstagingports.UploadInput) error {
	return nil
}

func (f *fakeUploadStagingForPublish) Delete(context.Context, uploadstagingports.DeleteInput) error {
	f.deleteCalls++
	return nil
}

func (*fakeUploadStagingForPublish) DeletePrefix(context.Context, uploadstagingports.DeletePrefixInput) error {
	return nil
}
