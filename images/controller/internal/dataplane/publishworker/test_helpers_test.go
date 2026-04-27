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
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type fakePublisher struct {
	onPublish func(modelpackports.PublishInput) error
}

func (f fakePublisher) Publish(_ context.Context, input modelpackports.PublishInput, _ modelpackports.RegistryAuth) (modelpackports.PublishResult, error) {
	if f.onPublish != nil {
		if err := f.onPublish(input); err != nil {
			return modelpackports.PublishResult{}, err
		}
	}
	return modelpackports.PublishResult{
		Reference: "registry.example.com/ai-models/catalog/model@sha256:deadbeef",
		Digest:    "sha256:deadbeef",
		MediaType: "application/vnd.cncf.model.manifest.v1+json",
		SizeBytes: 123,
	}, nil
}

type tarEntry struct {
	name    string
	content []byte
}

func createTestTar(path string, entries ...tarEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := tar.NewWriter(file)
	defer writer.Close()

	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.content))}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if _, err := writer.Write(entry.content); err != nil {
			return err
		}
	}
	return nil
}

func createTestZip(path string, entries ...tarEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	for _, entry := range entries {
		stream, err := writer.Create(entry.name)
		if err != nil {
			return err
		}
		if _, err := stream.Write(entry.content); err != nil {
			return err
		}
	}
	return nil
}

func writeTempFile(t *testing.T, name string, payload []byte) string {
	t.Helper()

	path := t.TempDir() + "/" + name
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

type fakeUploadStaging struct {
	payload             []byte
	objects             map[string][]byte
	httpClient          *http.Client
	downloadDestination string
	downloadCalls       int
	rangeCalls          int
	deleteCalls         int
}

func (f *fakeUploadStaging) HTTPClient() *http.Client {
	return f.httpClient
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

func (f *fakeUploadStaging) Stat(_ context.Context, input uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	payload, err := f.object(input.Key)
	if err != nil {
		return uploadstagingports.ObjectStat{}, err
	}
	return uploadstagingports.ObjectStat{SizeBytes: int64(len(payload)), ETag: `"stage-etag"`}, nil
}

func (f *fakeUploadStaging) Download(_ context.Context, input uploadstagingports.DownloadInput) error {
	f.downloadCalls++
	f.downloadDestination = input.DestinationPath
	payload, err := f.object(input.Key)
	if err != nil {
		return err
	}
	return os.WriteFile(input.DestinationPath, payload, 0o644)
}

func (f *fakeUploadStaging) OpenRead(_ context.Context, input uploadstagingports.OpenReadInput) (uploadstagingports.OpenReadOutput, error) {
	payload, err := f.object(input.Key)
	if err != nil {
		return uploadstagingports.OpenReadOutput{}, err
	}
	return uploadstagingports.OpenReadOutput{
		Body:      io.NopCloser(bytes.NewReader(payload)),
		SizeBytes: int64(len(payload)),
		ETag:      `"stage-etag"`,
	}, nil
}

func (f *fakeUploadStaging) OpenReadRange(_ context.Context, input uploadstagingports.OpenReadRangeInput) (uploadstagingports.OpenReadOutput, error) {
	f.rangeCalls++
	payload, err := f.object(input.Key)
	if err != nil {
		return uploadstagingports.OpenReadOutput{}, err
	}
	payload = sliceRange(payload, input.Offset, input.Length)
	return uploadstagingports.OpenReadOutput{
		Body:      io.NopCloser(bytes.NewReader(payload)),
		SizeBytes: int64(len(payload)),
		ETag:      `"stage-etag"`,
	}, nil
}

func (f *fakeUploadStaging) Upload(context.Context, uploadstagingports.UploadInput) error {
	return nil
}

func (f *fakeUploadStaging) Delete(context.Context, uploadstagingports.DeleteInput) error {
	f.deleteCalls++
	return nil
}

func (f *fakeUploadStaging) DeletePrefix(_ context.Context, input uploadstagingports.DeletePrefixInput) error {
	for key := range f.objects {
		if strings.HasPrefix(key, input.Prefix) {
			delete(f.objects, key)
		}
	}
	return nil
}

func (f *fakeUploadStaging) object(key string) ([]byte, error) {
	if f.objects == nil {
		return f.payload, nil
	}
	payload, found := f.objects[key]
	if !found {
		return nil, os.ErrNotExist
	}
	return payload, nil
}

func sliceRange(payload []byte, offset, length int64) []byte {
	start := offset
	if start < 0 {
		start = 0
	}
	if start > int64(len(payload)) {
		start = int64(len(payload))
	}
	end := int64(len(payload))
	if length >= 0 && start+length < end {
		end = start + length
	}
	if end < start {
		end = start
	}
	return payload[start:end]
}
