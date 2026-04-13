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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type fakeSourceMirrorStore struct {
	manifest sourcemirrorports.SnapshotManifest
	state    sourcemirrorports.SnapshotState
}

func (f *fakeSourceMirrorStore) SaveManifest(_ context.Context, manifest sourcemirrorports.SnapshotManifest) error {
	f.manifest = manifest
	return nil
}

func (f *fakeSourceMirrorStore) LoadManifest(context.Context, sourcemirrorports.SnapshotLocator) (sourcemirrorports.SnapshotManifest, error) {
	return f.manifest, nil
}

func (f *fakeSourceMirrorStore) SaveState(_ context.Context, state sourcemirrorports.SnapshotState) error {
	f.state = state
	return nil
}

func (f *fakeSourceMirrorStore) LoadState(context.Context, sourcemirrorports.SnapshotLocator) (sourcemirrorports.SnapshotState, error) {
	if f.state.Locator == (sourcemirrorports.SnapshotLocator{}) {
		return sourcemirrorports.SnapshotState{}, sourcemirrorports.ErrSnapshotNotFound
	}
	return f.state, nil
}

func (f *fakeSourceMirrorStore) DeleteSnapshot(context.Context, sourcemirrorports.SnapshotLocator) error {
	f.manifest = sourcemirrorports.SnapshotManifest{}
	f.state = sourcemirrorports.SnapshotState{}
	return nil
}

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
	uploadID := fmt.Sprintf("upload-%d", f.nextID)
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
			ETag:       fmt.Sprintf(`"etag-%d"`, partNumber),
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

func (f *fakeMirrorUploadStaging) Delete(_ context.Context, input uploadstagingports.DeleteInput) error {
	delete(f.objects, stagingObjectKey(input.Bucket, input.Key))
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

func (f *fakeMirrorUploadStaging) handleUploadPart(writer http.ResponseWriter, request *http.Request) {
	partNumber, uploadID, err := parseUploadPartPath(request.URL.Path)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	upload, found := f.uploads[uploadID]
	if !found {
		http.NotFound(writer, request)
		return
	}
	payload, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	upload.parts[partNumber] = payload
	writer.Header().Set("ETag", fmt.Sprintf(`"etag-%d"`, partNumber))
	writer.WriteHeader(http.StatusOK)
}

func parseUploadPartPath(rawPath string) (int32, string, error) {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(parts) != 3 || parts[0] != "multipart" {
		return 0, "", os.ErrNotExist
	}
	partNumber, err := strconv.ParseInt(parts[2], 10, 32)
	if err != nil {
		return 0, "", err
	}
	return int32(partNumber), parts[1], nil
}
