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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

type fakeBackend struct {
	objects          map[string][]byte
	uploads          map[string]string
	parts            map[string][]UploadedPart
	deleted          []string
	readerCalls      int
	putErr           error
	putErrPathSuffix string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		objects: make(map[string][]byte),
		uploads: make(map[string]string),
		parts:   make(map[string][]UploadedPart),
	}
}

func (b *fakeBackend) ObjectExists(_ context.Context, objectKey string) (bool, error) {
	_, exists := b.objects[strings.TrimSpace(objectKey)]
	return exists, nil
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
	b.parts[uploadID] = append([]UploadedPart(nil), parts...)
	b.objects[strings.TrimSpace(objectKey)] = payloadForParts(parts)
	return nil
}

func (b *fakeBackend) AbortMultipartUpload(_ context.Context, objectKey, uploadID string) error {
	delete(b.uploads, uploadID)
	delete(b.parts, uploadID)
	delete(b.objects, strings.TrimSpace(objectKey))
	return nil
}

func (b *fakeBackend) Reader(_ context.Context, objectKey string, offset int64) (io.ReadCloser, error) {
	b.readerCalls++
	payload, exists := b.objects[strings.TrimSpace(objectKey)]
	if !exists {
		return nil, errors.New("object not found")
	}
	if offset > int64(len(payload)) {
		offset = int64(len(payload))
	}
	return io.NopCloser(bytes.NewReader(payload[offset:])), nil
}

func (b *fakeBackend) DeleteObject(_ context.Context, objectKey string) error {
	trimmed := strings.TrimSpace(objectKey)
	delete(b.objects, trimmed)
	b.deleted = append(b.deleted, trimmed)
	return nil
}

func (b *fakeBackend) PutContent(_ context.Context, objectKey string, payload []byte) error {
	trimmed := strings.TrimSpace(objectKey)
	if b.putErr != nil && (b.putErrPathSuffix == "" || strings.HasSuffix(trimmed, b.putErrPathSuffix)) {
		return b.putErr
	}
	b.objects[trimmed] = append([]byte(nil), payload...)
	return nil
}

func TestServiceStartReturnsSessionToken(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	response := postJSON(t, server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{
		Repository: "ai-models/catalog/model",
	})
	if got, want := response.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var payload startResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}
	if payload.SessionToken == "" {
		t.Fatal("SessionToken = empty, want non-empty token")
	}
}

func TestServiceCompleteWritesRepositoryLinkAndSealedMetadata(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	startResp := postJSON(t, server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{
		Repository: "ai-models/catalog/model",
	})
	var startPayload startResponse
	if err := json.NewDecoder(startResp.Body).Decode(&startPayload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}
	claims, err := decodeSessionToken([]byte("salt"), startPayload.SessionToken)
	if err != nil {
		t.Fatalf("decodeSessionToken() error = %v", err)
	}

	parts := []UploadedPart{
		{PartNumber: 1, ETag: "etag-1", SizeBytes: 8},
		{PartNumber: 2, ETag: "etag-2", SizeBytes: 4},
	}
	digest := digestForParts(parts)
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       digest,
		SizeBytes:    12,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := string(backend.objects[linkKey]), digest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}

	blobKey, err := BlobDataObjectKey("/dmcr", digest)
	if err != nil {
		t.Fatalf("BlobDataObjectKey() error = %v", err)
	}
	if _, exists := backend.objects[blobKey]; exists {
		t.Fatalf("canonical blob object %q exists, want sealed metadata only", blobKey)
	}

	metadataPayload, exists := backend.objects[sealedblob.MetadataPath(blobKey)]
	if !exists {
		t.Fatalf("sealed metadata %q was not written", sealedblob.MetadataPath(blobKey))
	}
	metadata, err := sealedblob.Unmarshal(metadataPayload)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if metadata.Digest != digest {
		t.Fatalf("metadata.Digest = %q, want %q", metadata.Digest, digest)
	}
	if metadata.PhysicalPath != claims.ObjectKey {
		t.Fatalf("metadata.PhysicalPath = %q, want %q", metadata.PhysicalPath, claims.ObjectKey)
	}
	if metadata.SizeBytes != 12 {
		t.Fatalf("metadata.SizeBytes = %d, want %d", metadata.SizeBytes, 12)
	}
	if _, exists := backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist", claims.ObjectKey)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for trusted sealing path", got)
	}
}

func TestServiceCompleteUsesTrustedDigestWithoutReread(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	startResp := postJSON(t, server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{
		Repository: "ai-models/catalog/model",
	})
	var startPayload startResponse
	if err := json.NewDecoder(startResp.Body).Decode(&startPayload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}
	claims, err := decodeSessionToken([]byte("salt"), startPayload.SessionToken)
	if err != nil {
		t.Fatalf("decodeSessionToken() error = %v", err)
	}

	parts := []UploadedPart{
		{PartNumber: 1, ETag: "etag-1", SizeBytes: 8},
	}
	trustedDigest := "sha256:" + strings.Repeat("f", 64)
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       trustedDigest,
		SizeBytes:    8,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if slices.Contains(backend.deleted, claims.ObjectKey) {
		t.Fatalf("expected physical upload object %q to be retained, deleted = %#v", claims.ObjectKey, backend.deleted)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for trusted sealing path", got)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", trustedDigest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got := string(backend.objects[linkKey]); got != trustedDigest {
		t.Fatalf("link payload = %q, want %q", got, trustedDigest)
	}
	if _, exists := backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist", claims.ObjectKey)
	}
}

func TestServiceCompleteRejectsMissingDigest(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	startResp := postJSON(t, server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{
		Repository: "ai-models/catalog/model",
	})
	var startPayload startResponse
	if err := json.NewDecoder(startResp.Body).Decode(&startPayload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}

	parts := []UploadedPart{
		{PartNumber: 1, ETag: "etag-1", SizeBytes: 8},
	}
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		SizeBytes:    8,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestServiceRejectsWrongAuth(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	response := postJSON(t, server.URL+"/v2/blob-uploads", "writer", "wrong", startRequest{
		Repository: "ai-models/catalog/model",
	})
	if got, want := response.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestServiceRejectsExpiredSessionToken(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	startedAt := time.Unix(1_700_000_000, 0)
	service.now = func() time.Time { return startedAt }

	server := httptest.NewServer(service.Handler())
	defer server.Close()

	startResp := postJSON(t, server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{
		Repository: "ai-models/catalog/model",
	})
	var startPayload startResponse
	if err := json.NewDecoder(startResp.Body).Decode(&startPayload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}

	service.now = func() time.Time { return startedAt.Add(2 * time.Hour) }

	request, err := http.NewRequest(http.MethodGet, server.URL+"/v2/blob-uploads/parts?sessionToken="+startPayload.SessionToken, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.SetBasicAuth("writer", "secret")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	if got, want := response.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestServiceCompleteCleansUpSealedObjectsWhenLinkWriteFails(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	backend.putErr = errors.New("link write failed")
	backend.putErrPathSuffix = "/link"
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	startResp := postJSON(t, server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{
		Repository: "ai-models/catalog/model",
	})
	var startPayload startResponse
	if err := json.NewDecoder(startResp.Body).Decode(&startPayload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}
	claims, err := decodeSessionToken([]byte("salt"), startPayload.SessionToken)
	if err != nil {
		t.Fatalf("decodeSessionToken() error = %v", err)
	}

	parts := []UploadedPart{
		{PartNumber: 1, ETag: "etag-1", SizeBytes: 8},
	}
	digest := digestForParts(parts)
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       digest,
		SizeBytes:    8,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	blobKey, err := BlobDataObjectKey("/dmcr", digest)
	if err != nil {
		t.Fatalf("BlobDataObjectKey() error = %v", err)
	}
	expectedDeleted := []string{claims.ObjectKey, sealedblob.MetadataPath(blobKey)}
	for _, expectedPath := range expectedDeleted {
		if !slices.Contains(backend.deleted, expectedPath) {
			t.Fatalf("DeleteObject() did not remove %q, deleted = %#v", expectedPath, backend.deleted)
		}
		if _, exists := backend.objects[expectedPath]; exists {
			t.Fatalf("object %q still exists after cleanup", expectedPath)
		}
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

func payloadForParts(parts []UploadedPart) []byte {
	payload := make([]byte, 0, int(totalUploadedSize(parts)))
	for _, part := range parts {
		payload = append(payload, bytes.Repeat([]byte{byte('a' + part.PartNumber - 1)}, int(part.SizeBytes))...)
	}
	return payload
}

func digestForParts(parts []UploadedPart) string {
	sum := sha256.Sum256(payloadForParts(parts))
	return "sha256:" + hex.EncodeToString(sum[:])
}
