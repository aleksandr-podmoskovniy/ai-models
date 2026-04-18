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
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type fakeMirrorUploadStaging struct {
	serverURL string
	objects   map[string][]byte
	uploads   map[string]*fakeMultipartUpload
	nextID    int
}

type fakeMultipartUpload struct {
	parts map[int32][]byte
}

func newFakeMirrorUploadStaging(t interface{ Cleanup(func()) }) *fakeMirrorUploadStaging {
	client := &fakeMirrorUploadStaging{
		objects: make(map[string][]byte),
		uploads: make(map[string]*fakeMultipartUpload),
	}
	server := httptest.NewServer(http.HandlerFunc(client.handleUploadPart))
	client.serverURL = server.URL
	t.Cleanup(server.Close)
	return client
}

func newFakeMirrorUploadStagingTLS(t interface{ Cleanup(func()) }) (*fakeMirrorUploadStaging, *http.Client) {
	client := &fakeMirrorUploadStaging{
		objects: make(map[string][]byte),
		uploads: make(map[string]*fakeMultipartUpload),
	}
	server := httptest.NewTLSServer(http.HandlerFunc(client.handleUploadPart))
	client.serverURL = server.URL
	t.Cleanup(server.Close)
	return client, server.Client()
}

func (f *fakeMirrorUploadStaging) StartMultipartUpload(_ context.Context, input uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	f.nextID++
	uploadID := "upload-" + strconv.Itoa(f.nextID)
	f.seedMultipartUpload(uploadID)
	return uploadstagingports.StartMultipartUploadOutput{UploadID: uploadID}, nil
}

func (f *fakeMirrorUploadStaging) PresignUploadPart(_ context.Context, input uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{
		URL: f.serverURL + "/multipart/" + input.UploadID + "/" + strconv.FormatInt(int64(input.PartNumber), 10),
	}, nil
}

func (f *fakeMirrorUploadStaging) ListMultipartUploadParts(_ context.Context, input uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	upload, found := f.uploads[input.UploadID]
	if !found {
		return nil, os.ErrNotExist
	}
	partNumbers := make([]int32, 0, len(upload.parts))
	for partNumber := range upload.parts {
		partNumbers = append(partNumbers, partNumber)
	}
	slices.Sort(partNumbers)
	result := make([]uploadstagingports.UploadedPart, 0, len(partNumbers))
	for _, partNumber := range partNumbers {
		result = append(result, uploadstagingports.UploadedPart{
			PartNumber: partNumber,
			ETag:       `"etag-` + strconv.Itoa(int(partNumber)) + `"`,
			SizeBytes:  int64(len(upload.parts[partNumber])),
		})
	}
	return result, nil
}

func (f *fakeMirrorUploadStaging) CompleteMultipartUpload(_ context.Context, input uploadstagingports.CompleteMultipartUploadInput) error {
	upload, found := f.uploads[input.UploadID]
	if !found {
		return os.ErrNotExist
	}
	payload := make([]byte, 0)
	for _, part := range input.Parts {
		payload = append(payload, upload.parts[part.PartNumber]...)
	}
	f.objects[stagingObjectKey(input.Bucket, input.Key)] = payload
	delete(f.uploads, input.UploadID)
	return nil
}

func (f *fakeMirrorUploadStaging) AbortMultipartUpload(_ context.Context, input uploadstagingports.AbortMultipartUploadInput) error {
	delete(f.uploads, input.UploadID)
	return nil
}

func (f *fakeMirrorUploadStaging) Upload(_ context.Context, input uploadstagingports.UploadInput) error {
	payload, err := io.ReadAll(input.Body)
	if err != nil {
		return err
	}
	f.objects[stagingObjectKey(input.Bucket, input.Key)] = payload
	return nil
}

func (f *fakeMirrorUploadStaging) Stat(_ context.Context, input uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	payload, found := f.objects[stagingObjectKey(input.Bucket, input.Key)]
	if !found {
		return uploadstagingports.ObjectStat{}, os.ErrNotExist
	}
	return uploadstagingports.ObjectStat{SizeBytes: int64(len(payload)), ETag: `"etag-complete"`}, nil
}

func (f *fakeMirrorUploadStaging) Download(_ context.Context, input uploadstagingports.DownloadInput) error {
	payload, found := f.objects[stagingObjectKey(input.Bucket, input.Key)]
	if !found {
		return os.ErrNotExist
	}
	return os.WriteFile(input.DestinationPath, payload, 0o644)
}

func (f *fakeMirrorUploadStaging) OpenRead(_ context.Context, input uploadstagingports.OpenReadInput) (uploadstagingports.OpenReadOutput, error) {
	payload, found := f.objects[stagingObjectKey(input.Bucket, input.Key)]
	if !found {
		return uploadstagingports.OpenReadOutput{}, os.ErrNotExist
	}
	return uploadstagingports.OpenReadOutput{
		Body:      io.NopCloser(bytes.NewReader(payload)),
		SizeBytes: int64(len(payload)),
		ETag:      `"etag-complete"`,
	}, nil
}

func (f *fakeMirrorUploadStaging) OpenReadRange(_ context.Context, input uploadstagingports.OpenReadRangeInput) (uploadstagingports.OpenReadOutput, error) {
	payload, found := f.objects[stagingObjectKey(input.Bucket, input.Key)]
	if !found {
		return uploadstagingports.OpenReadOutput{}, os.ErrNotExist
	}
	if input.Offset < 0 || input.Offset > int64(len(payload)) {
		return uploadstagingports.OpenReadOutput{}, io.EOF
	}
	end := int64(len(payload))
	if input.Length >= 0 && input.Offset+input.Length < end {
		end = input.Offset + input.Length
	}
	chunk := append([]byte(nil), payload[int(input.Offset):int(end)]...)
	return uploadstagingports.OpenReadOutput{
		Body:      io.NopCloser(bytes.NewReader(chunk)),
		SizeBytes: int64(len(chunk)),
		ETag:      `"etag-complete"`,
	}, nil
}

func (f *fakeMirrorUploadStaging) Delete(_ context.Context, input uploadstagingports.DeleteInput) error {
	delete(f.objects, stagingObjectKey(input.Bucket, input.Key))
	return nil
}

func (f *fakeMirrorUploadStaging) DeletePrefix(_ context.Context, input uploadstagingports.DeletePrefixInput) error {
	prefix := stagingObjectKey(input.Bucket, input.Prefix)
	for key := range f.objects {
		if strings.HasPrefix(key, prefix) {
			delete(f.objects, key)
		}
	}
	return nil
}

func (f *fakeMirrorUploadStaging) seedMultipartUpload(uploadID string) {
	f.uploads[uploadID] = &fakeMultipartUpload{parts: make(map[int32][]byte)}
}

func (f *fakeMirrorUploadStaging) seedUploadedPart(uploadID string, partNumber int32, payload []byte) {
	if upload, found := f.uploads[uploadID]; found {
		upload.parts[partNumber] = append([]byte(nil), payload...)
	}
}
