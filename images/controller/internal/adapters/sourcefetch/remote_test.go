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

package sourcefetch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func TestFetchRemoteModelHTTPGGUF(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/model.gguf" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte("GGUFpayload"))
	}))
	defer server.Close()

	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:       server.URL + "/model.gguf",
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}
	if got, want := result.SourceType, modelsv1alpha1.ModelSourceTypeHTTP; got != want {
		t.Fatalf("unexpected source type %q", got)
	}
	if got, want := result.InputFormat, modelsv1alpha1.ModelInputFormatGGUF; got != want {
		t.Fatalf("unexpected input format %q", got)
	}
	if _, err := os.Stat(filepath.Join(result.ModelDir, "model.gguf")); err != nil {
		t.Fatalf("Stat(model.gguf) error = %v", err)
	}
}

func TestFetchRemoteModelHTTPStagesRawObjectBeforePreparingCheckpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/model.gguf" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte("GGUFpayload"))
	}))
	defer server.Close()

	staging := newFakeUploadStaging()
	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:       server.URL + "/model.gguf",
		Workspace: t.TempDir(),
		RawStage: &RawStageOptions{
			Bucket:    "artifacts",
			KeyPrefix: "raw/1111-2222/source-url",
			Client:    staging,
		},
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}
	if got, want := len(result.StagedObjects), 1; got != want {
		t.Fatalf("unexpected staged object count %d", got)
	}
	if got, want := result.StagedObjects[0].Key, "raw/1111-2222/source-url/model.gguf"; got != want {
		t.Fatalf("unexpected staged object key %q", got)
	}
	if got, want := string(staging.objects["artifacts/raw/1111-2222/source-url/model.gguf"]), "GGUFpayload"; got != want {
		t.Fatalf("unexpected staged raw payload %q", got)
	}
	if _, err := os.Stat(filepath.Join(result.ModelDir, "model.gguf")); err != nil {
		t.Fatalf("Stat(model.gguf) error = %v", err)
	}
}

type fakeUploadStaging struct {
	objects map[string][]byte
}

func newFakeUploadStaging() *fakeUploadStaging {
	return &fakeUploadStaging{objects: map[string][]byte{}}
}

func (f *fakeUploadStaging) StartMultipartUpload(context.Context, uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	return uploadstagingports.StartMultipartUploadOutput{}, nil
}

func (f *fakeUploadStaging) PresignUploadPart(context.Context, uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{}, nil
}

func (f *fakeUploadStaging) ListMultipartUploadParts(context.Context, uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	return nil, nil
}

func (f *fakeUploadStaging) CompleteMultipartUpload(context.Context, uploadstagingports.CompleteMultipartUploadInput) error {
	return nil
}

func (f *fakeUploadStaging) AbortMultipartUpload(context.Context, uploadstagingports.AbortMultipartUploadInput) error {
	return nil
}

func (f *fakeUploadStaging) Stat(context.Context, uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	return uploadstagingports.ObjectStat{}, nil
}

func (f *fakeUploadStaging) Upload(_ context.Context, input uploadstagingports.UploadInput) error {
	payload, err := io.ReadAll(input.Body)
	if err != nil {
		return err
	}
	f.objects[stagingObjectKey(input.Bucket, input.Key)] = payload
	return nil
}

func (f *fakeUploadStaging) Download(_ context.Context, input uploadstagingports.DownloadInput) error {
	payload := f.objects[stagingObjectKey(input.Bucket, input.Key)]
	return os.WriteFile(input.DestinationPath, payload, 0o644)
}

func (f *fakeUploadStaging) Delete(_ context.Context, input uploadstagingports.DeleteInput) error {
	delete(f.objects, stagingObjectKey(input.Bucket, input.Key))
	return nil
}

func stagingObjectKey(bucket, key string) string {
	return bucket + "/" + key
}
