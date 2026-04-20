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
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type fakeUploadStagingWithHTTPClient struct {
	httpClient *http.Client
}

func (f *fakeUploadStagingWithHTTPClient) HTTPClient() *http.Client {
	return f.httpClient
}

func (*fakeUploadStagingWithHTTPClient) StartMultipartUpload(context.Context, uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	return uploadstagingports.StartMultipartUploadOutput{}, nil
}

func (*fakeUploadStagingWithHTTPClient) PresignUploadPart(context.Context, uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{}, nil
}

func (*fakeUploadStagingWithHTTPClient) ListMultipartUploadParts(context.Context, uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	return nil, nil
}

func (*fakeUploadStagingWithHTTPClient) CompleteMultipartUpload(context.Context, uploadstagingports.CompleteMultipartUploadInput) error {
	return nil
}

func (*fakeUploadStagingWithHTTPClient) AbortMultipartUpload(context.Context, uploadstagingports.AbortMultipartUploadInput) error {
	return nil
}

func (*fakeUploadStagingWithHTTPClient) Stat(context.Context, uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	return uploadstagingports.ObjectStat{}, nil
}

func (*fakeUploadStagingWithHTTPClient) OpenRead(context.Context, uploadstagingports.OpenReadInput) (uploadstagingports.OpenReadOutput, error) {
	return uploadstagingports.OpenReadOutput{Body: io.NopCloser(http.NoBody)}, nil
}

func (*fakeUploadStagingWithHTTPClient) Download(context.Context, uploadstagingports.DownloadInput) error {
	return nil
}

func (*fakeUploadStagingWithHTTPClient) Upload(context.Context, uploadstagingports.UploadInput) error {
	return nil
}

func (*fakeUploadStagingWithHTTPClient) Delete(context.Context, uploadstagingports.DeleteInput) error {
	return nil
}

func (*fakeUploadStagingWithHTTPClient) DeletePrefix(context.Context, uploadstagingports.DeletePrefixInput) error {
	return nil
}

func TestRemoteSourceMirrorPropagatesUploadHTTPClient(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Timeout: time.Second}
	staging := &fakeUploadStagingWithHTTPClient{httpClient: httpClient}

	options := remoteSourceMirror(Options{
		SourceFetchMode: publicationports.SourceFetchModeMirror,
		RawStageBucket:        "artifacts",
		RawStageKeyPrefix:     "raw/1111-2222/source-url",
		UploadStaging:         staging,
	})
	if options == nil {
		t.Fatal("expected source mirror options")
	}
	if got, want := options.UploadHTTPClient, httpClient; got != want {
		t.Fatalf("unexpected upload HTTP client %p", got)
	}
}

func TestRemoteSourceMirrorDisabledForDirectMode(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Timeout: time.Second}
	staging := &fakeUploadStagingWithHTTPClient{httpClient: httpClient}

	options := remoteSourceMirror(Options{
		SourceFetchMode: publicationports.SourceFetchModeDirect,
		RawStageBucket:        "artifacts",
		RawStageKeyPrefix:     "raw/1111-2222/source-url",
		UploadStaging:         staging,
	})
	if options != nil {
		t.Fatalf("expected no source mirror options in direct mode, got %#v", options)
	}
}
