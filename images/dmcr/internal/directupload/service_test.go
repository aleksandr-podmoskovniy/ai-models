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

package directupload

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type fakeBackend struct {
	existing map[string]bool
	uploads  map[string]string
	links    map[string]string
	parts    map[string][]UploadedPart
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		existing: make(map[string]bool),
		uploads:  make(map[string]string),
		links:    make(map[string]string),
		parts:    make(map[string][]UploadedPart),
	}
}

func (b *fakeBackend) BlobExists(_ context.Context, objectKey string) (bool, error) {
	return b.existing[objectKey], nil
}

func (b *fakeBackend) StartMultipartUpload(_ context.Context, objectKey string) (string, error) {
	uploadID := "upload-" + objectKey
	b.uploads[uploadID] = objectKey
	return uploadID, nil
}

func (b *fakeBackend) PresignUploadPart(_ context.Context, _, uploadID string, partNumber int) (string, error) {
	return "https://upload.example/" + uploadID + "/" + strconv.Itoa(partNumber), nil
}

func (b *fakeBackend) ListUploadedParts(_ context.Context, objectKey, uploadID string) ([]UploadedPart, error) {
	_ = objectKey
	return append([]UploadedPart(nil), b.parts[uploadID]...), nil
}

func (b *fakeBackend) CompleteMultipartUpload(_ context.Context, objectKey, uploadID string, parts []UploadedPart) error {
	b.existing[objectKey] = true
	b.parts[uploadID] = append([]UploadedPart(nil), parts...)
	return nil
}

func (b *fakeBackend) AbortMultipartUpload(_ context.Context, objectKey, uploadID string) error {
	delete(b.uploads, uploadID)
	delete(b.parts, uploadID)
	delete(b.existing, objectKey)
	return nil
}

func (b *fakeBackend) PutContent(_ context.Context, objectKey string, payload []byte) error {
	b.links[objectKey] = string(payload)
	return nil
}

func TestServiceExistingBlobCompletesImmediatelyAndWritesLink(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	blobKey, err := BlobDataObjectKey("/dmcr", "sha256:"+strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("BlobDataObjectKey() error = %v", err)
	}
	backend.existing[blobKey] = true
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	response := postJSON(t, server.URL+"/v1/blob-uploads", "writer", "secret", startRequest{
		Repository: "ai-models/catalog/model",
		Digest:     "sha256:" + strings.Repeat("a", 64),
	})
	if got, want := response.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var payload startResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}
	if !payload.Complete {
		t.Fatal("Complete = false, want true")
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", "sha256:"+strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := backend.links[linkKey], "sha256:"+strings.Repeat("a", 64); got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
}

func TestServiceCompleteWritesRepositoryLink(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	startResp := postJSON(t, server.URL+"/v1/blob-uploads", "writer", "secret", startRequest{
		Repository: "ai-models/catalog/model",
		Digest:     "sha256:" + strings.Repeat("b", 64),
	})
	var startPayload startResponse
	if err := json.NewDecoder(startResp.Body).Decode(&startPayload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}
	if startPayload.SessionToken == "" {
		t.Fatal("SessionToken = empty, want non-empty token")
	}

	completeResp := postJSON(t, server.URL+"/v1/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Parts: []UploadedPart{
			{PartNumber: 1, ETag: "etag-1", SizeBytes: 8},
			{PartNumber: 2, ETag: "etag-2", SizeBytes: 4},
		},
	})
	if got, want := completeResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", "sha256:"+strings.Repeat("b", 64))
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := backend.links[linkKey], "sha256:"+strings.Repeat("b", 64); got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
}

func TestServiceRejectsWrongAuth(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	response := postJSON(t, server.URL+"/v1/blob-uploads", "writer", "wrong", startRequest{
		Repository: "ai-models/catalog/model",
		Digest:     "sha256:" + strings.Repeat("c", 64),
	})
	if got, want := response.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func postJSON(t *testing.T, url, username, password string, payload any) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(username, password)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	})
	return response
}
