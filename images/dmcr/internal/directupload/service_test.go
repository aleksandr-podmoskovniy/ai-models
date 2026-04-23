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
	"encoding/base64"
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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

type fakeBackend struct {
	objects          map[string][]byte
	attributes       map[string]ObjectAttributes
	uploads          map[string]string
	parts            map[string][]UploadedPart
	deleted          []string
	attributesCalls  int
	attributesErr    error
	readerCalls      int
	completeErr      error
	readerErr        error
	putErr           error
	putErrPathSuffix string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		objects:    make(map[string][]byte),
		attributes: make(map[string]ObjectAttributes),
		uploads:    make(map[string]string),
		parts:      make(map[string][]UploadedPart),
	}
}

func (b *fakeBackend) ObjectExists(_ context.Context, objectKey string) (bool, error) {
	_, exists := b.objects[strings.TrimSpace(objectKey)]
	return exists, nil
}

func (b *fakeBackend) ObjectAttributes(_ context.Context, objectKey string) (ObjectAttributes, error) {
	b.attributesCalls++
	if b.attributesErr != nil {
		return ObjectAttributes{}, b.attributesErr
	}
	trimmed := strings.TrimSpace(objectKey)
	if attributes, exists := b.attributes[trimmed]; exists {
		return attributes, nil
	}
	payload, exists := b.objects[trimmed]
	if !exists {
		return ObjectAttributes{}, errors.New("object not found")
	}
	return ObjectAttributes{SizeBytes: int64(len(payload))}, nil
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
	if b.completeErr != nil {
		return b.completeErr
	}
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
	if b.readerErr != nil {
		return nil, b.readerErr
	}
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
	var completePayload completeResponse
	if err := json.NewDecoder(completeResp.Body).Decode(&completePayload); err != nil {
		t.Fatalf("Decode(completeResponse) error = %v", err)
	}
	if completePayload.Digest != digest {
		t.Fatalf("complete digest = %q, want %q", completePayload.Digest, digest)
	}
	if completePayload.SizeBytes != 12 {
		t.Fatalf("complete sizeBytes = %d, want 12", completePayload.SizeBytes)
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
	expectedPhysicalPath := storageDriverPathForObjectKey("/dmcr", claims.ObjectKey)
	if metadata.PhysicalPath != expectedPhysicalPath {
		t.Fatalf("metadata.PhysicalPath = %q, want %q", metadata.PhysicalPath, expectedPhysicalPath)
	}
	if metadata.SizeBytes != 12 {
		t.Fatalf("metadata.SizeBytes = %d, want %d", metadata.SizeBytes, 12)
	}
	if _, exists := backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist", claims.ObjectKey)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
}

func TestServiceCompleteUsesTrustedBackendDigestWithoutReadingObject(t *testing.T) {
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
	backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                     12,
		TrustedFullObjectSHA256Digest: digest,
		ReportedChecksumType:          checksumTypeFullObject,
		SHA256ChecksumPresent:         true,
		AvailableChecksumAlgorithms:   []string{"SHA256"},
	}

	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       digest,
		SizeBytes:    12,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got := backend.attributesCalls; got != 1 {
		t.Fatalf("ObjectAttributes() call count = %d, want 1", got)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for trusted backend digest path", got)
	}
}

func TestTrustedFullObjectSHA256DigestAcceptsFullObjectChecksum(t *testing.T) {
	t.Parallel()

	checksum := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("\xaa", sha256DigestBytes)))

	got := trustedFullObjectSHA256Digest(aws.String(checksum), types.ChecksumTypeFullObject)
	want := "sha256:" + strings.Repeat("aa", sha256DigestBytes)
	if got != want {
		t.Fatalf("trustedFullObjectSHA256Digest() = %q, want %q", got, want)
	}
}

func TestTrustedFullObjectSHA256DigestRejectsCompositeChecksum(t *testing.T) {
	t.Parallel()

	checksum := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("\xbb", sha256DigestBytes)))

	got := trustedFullObjectSHA256Digest(aws.String(checksum), types.ChecksumTypeComposite)
	if got != "" {
		t.Fatalf("trustedFullObjectSHA256Digest() = %q, want empty digest", got)
	}
}

func TestTrustedFullObjectSHA256DigestRejectsMalformedChecksum(t *testing.T) {
	t.Parallel()

	got := trustedFullObjectSHA256Digest(aws.String("not-base64"), types.ChecksumTypeFullObject)
	if got != "" {
		t.Fatalf("trustedFullObjectSHA256Digest() = %q, want empty digest", got)
	}
}

func TestServiceCompleteTrustsClientDigestWhenBackendDigestLookupFailsByDefault(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	backend.attributesErr = errors.New("checksum metadata is not supported")
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
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
}

func TestServiceCompleteTrustsClientDigestWhenTrustedBackendDigestIsMalformedByDefault(t *testing.T) {
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
	backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                     12,
		TrustedFullObjectSHA256Digest: "sha256:not-a-valid-digest",
		ReportedChecksumType:          checksumTypeFullObject,
		SHA256ChecksumPresent:         true,
		AvailableChecksumAlgorithms:   []string{"SHA256"},
	}
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       digest,
		SizeBytes:    12,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
}

func TestServiceCompleteRejectsTrustedBackendSizeMismatch(t *testing.T) {
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
	backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                     11,
		TrustedFullObjectSHA256Digest: digest,
		ReportedChecksumType:          checksumTypeFullObject,
		SHA256ChecksumPresent:         true,
		AvailableChecksumAlgorithms:   []string{"SHA256"},
	}
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       digest,
		SizeBytes:    12,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusConflict; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if !slices.Contains(backend.deleted, claims.ObjectKey) {
		t.Fatalf("expected physical upload object %q to be deleted, deleted = %#v", claims.ObjectKey, backend.deleted)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if _, exists := backend.objects[linkKey]; exists {
		t.Fatalf("repository link %q exists after trusted size mismatch", linkKey)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 when trusted size mismatches", got)
	}
}

func TestServiceCompleteRejectsTrustedBackendDigestMismatch(t *testing.T) {
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
	expectedDigest := digestForParts(parts)
	backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                     12,
		TrustedFullObjectSHA256Digest: "sha256:" + strings.Repeat("f", 64),
		ReportedChecksumType:          checksumTypeFullObject,
		SHA256ChecksumPresent:         true,
		AvailableChecksumAlgorithms:   []string{"SHA256"},
	}
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       expectedDigest,
		SizeBytes:    12,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusConflict; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if !slices.Contains(backend.deleted, claims.ObjectKey) {
		t.Fatalf("expected physical upload object %q to be deleted, deleted = %#v", claims.ObjectKey, backend.deleted)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", expectedDigest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if _, exists := backend.objects[linkKey]; exists {
		t.Fatalf("repository link %q exists after trusted digest mismatch", linkKey)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 when trusted digest mismatches", got)
	}
}

func TestServiceCompleteTrustsExpectedDigestWithoutTrustedBackendChecksumByDefault(t *testing.T) {
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
		t.Fatalf("physical upload object %q was deleted, deleted = %#v", claims.ObjectKey, backend.deleted)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", trustedDigest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := string(backend.objects[linkKey]), trustedDigest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
	if _, exists := backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist", claims.ObjectKey)
	}
}

func TestServiceCompleteComputesDigestWithoutClientDigest(t *testing.T) {
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
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		SizeBytes:    8,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var completePayload completeResponse
	if err := json.NewDecoder(completeResp.Body).Decode(&completePayload); err != nil {
		t.Fatalf("Decode(completeResponse) error = %v", err)
	}
	digest := digestForParts(parts)
	if completePayload.Digest != digest {
		t.Fatalf("complete digest = %q, want %q", completePayload.Digest, digest)
	}
	if completePayload.SizeBytes != 8 {
		t.Fatalf("complete sizeBytes = %d, want 8", completePayload.SizeBytes)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := string(backend.objects[linkKey]), digest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
	if _, exists := backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist", claims.ObjectKey)
	}
	if got := backend.readerCalls; got != 1 {
		t.Fatalf("Reader() call count = %d, want 1 when client digest is absent", got)
	}
}

func TestTrustedBackendVerificationReportsMissingChecksumFallback(t *testing.T) {
	t.Parallel()

	_, ok, reason, err := trustedBackendVerification(ObjectAttributes{
		SizeBytes: 128,
	})
	if err != nil {
		t.Fatalf("trustedBackendVerification() error = %v", err)
	}
	if ok {
		t.Fatal("trustedBackendVerification() ok = true, want false")
	}
	if reason != verificationFallbackReasonChecksumMissing {
		t.Fatalf("trustedBackendVerification() reason = %q, want %q", reason, verificationFallbackReasonChecksumMissing)
	}
}

func TestTrustedBackendVerificationReportsCompositeChecksumFallback(t *testing.T) {
	t.Parallel()

	_, ok, reason, err := trustedBackendVerification(ObjectAttributes{
		SizeBytes:             128,
		ReportedChecksumType:  "COMPOSITE",
		SHA256ChecksumPresent: true,
	})
	if err != nil {
		t.Fatalf("trustedBackendVerification() error = %v", err)
	}
	if ok {
		t.Fatal("trustedBackendVerification() ok = true, want false")
	}
	if reason != verificationFallbackReasonChecksumComposite {
		t.Fatalf("trustedBackendVerification() reason = %q, want %q", reason, verificationFallbackReasonChecksumComposite)
	}
}

func TestTrustedBackendVerificationReportsMalformedChecksumFallback(t *testing.T) {
	t.Parallel()

	_, ok, reason, err := trustedBackendVerification(ObjectAttributes{
		SizeBytes:             128,
		ReportedChecksumType:  checksumTypeFullObject,
		SHA256ChecksumPresent: true,
	})
	if err != nil {
		t.Fatalf("trustedBackendVerification() error = %v", err)
	}
	if ok {
		t.Fatal("trustedBackendVerification() ok = true, want false")
	}
	if reason != verificationFallbackReasonChecksumMalformed {
		t.Fatalf("trustedBackendVerification() reason = %q, want %q", reason, verificationFallbackReasonChecksumMalformed)
	}
}

func TestParseVerificationPolicyDefaultsToClientAsserted(t *testing.T) {
	t.Parallel()

	got, err := ParseVerificationPolicy("")
	if err != nil {
		t.Fatalf("ParseVerificationPolicy() error = %v", err)
	}
	if got != VerificationPolicyTrustedBackendOrClientAsserted {
		t.Fatalf("ParseVerificationPolicy() = %q, want %q", got, VerificationPolicyTrustedBackendOrClientAsserted)
	}
}

func TestParseVerificationPolicyRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	if _, err := ParseVerificationPolicy("unknown"); err == nil {
		t.Fatal("ParseVerificationPolicy() error = nil, want non-nil")
	}
}

func TestVerificationReadProgressWriterLogsOnByteThreshold(t *testing.T) {
	t.Parallel()

	startedAt := time.Unix(1_700_000_000, 0).UTC()
	now := startedAt
	messages := make([]string, 0, 2)

	writer := newVerificationReadProgressWriter("dmcr/_ai_models/direct-upload/objects/session/data", 2<<30, startedAt)
	writer.now = func() time.Time { return now }
	writer.emit = func(format string, args ...any) {
		messages = append(messages, format)
	}

	if _, err := writer.Write(bytes.Repeat([]byte("a"), int(verificationReadProgressStepBytes))); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("log count = %d, want 1", len(messages))
	}
	if writer.nextProgressBytes != verificationReadProgressStepBytes*2 {
		t.Fatalf("nextProgressBytes = %d, want %d", writer.nextProgressBytes, verificationReadProgressStepBytes*2)
	}
}

func TestVerificationReadProgressWriterLogsOnTimeInterval(t *testing.T) {
	t.Parallel()

	startedAt := time.Unix(1_700_000_000, 0).UTC()
	now := startedAt
	messages := make([]string, 0, 2)

	writer := newVerificationReadProgressWriter("dmcr/_ai_models/direct-upload/objects/session/data", 768<<20, startedAt)
	writer.now = func() time.Time { return now }
	writer.emit = func(format string, args ...any) {
		messages = append(messages, format)
	}

	if _, err := writer.Write(bytes.Repeat([]byte("b"), 8<<20)); err != nil {
		t.Fatalf("first Write() error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("log count after first write = %d, want 0", len(messages))
	}

	now = startedAt.Add(verificationReadProgressInterval)
	if _, err := writer.Write(bytes.Repeat([]byte("c"), 8<<20)); err != nil {
		t.Fatalf("second Write() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("log count after second write = %d, want 1", len(messages))
	}
}

func TestServiceCompleteVerifiesAlreadyCompletedObject(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	backend.completeErr = errors.New("multipart upload is already completed")
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
	backend.objects[claims.ObjectKey] = payloadForParts(parts)

	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       digest,
		SizeBytes:    8,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := string(backend.objects[linkKey]), digest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
}

func TestServiceCompleteStrictPolicyKeepsPhysicalObjectWhenVerificationReadFails(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	backend.readerErr = errors.New("temporary read failure")
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	mustSetVerificationPolicy(t, service, VerificationPolicyTrustedBackendOrReread)
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
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       digestForParts(parts),
		SizeBytes:    8,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if slices.Contains(backend.deleted, claims.ObjectKey) {
		t.Fatalf("physical upload object %q was deleted after temporary verification failure", claims.ObjectKey)
	}
	if _, exists := backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist after temporary verification failure", claims.ObjectKey)
	}
}

func TestServiceCompleteStrictPolicyFallsBackWhenBackendDigestLookupFails(t *testing.T) {
	t.Parallel()

	backend := newFakeBackend()
	backend.attributesErr = errors.New("checksum metadata is not supported")
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	mustSetVerificationPolicy(t, service, VerificationPolicyTrustedBackendOrReread)
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
	if got := backend.readerCalls; got != 1 {
		t.Fatalf("Reader() call count = %d, want 1 for strict reread policy", got)
	}
}

func TestServiceCompleteRejectsBackendSizeMismatchWithoutChecksumByDefault(t *testing.T) {
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
	backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                   11,
		ReportedChecksumType:        "",
		SHA256ChecksumPresent:       false,
		AvailableChecksumAlgorithms: nil,
	}
	completeResp := postJSON(t, server.URL+"/v2/blob-uploads/complete", "writer", "secret", completeRequest{
		SessionToken: startPayload.SessionToken,
		Digest:       digest,
		SizeBytes:    12,
		Parts:        parts,
	})
	if got, want := completeResp.StatusCode, http.StatusConflict; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if !slices.Contains(backend.deleted, claims.ObjectKey) {
		t.Fatalf("expected physical upload object %q to be deleted, deleted = %#v", claims.ObjectKey, backend.deleted)
	}
	if got := backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 without reread in default policy", got)
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

func mustSetVerificationPolicy(t *testing.T, service *Service, policy VerificationPolicy) {
	t.Helper()
	if err := service.SetVerificationPolicy(policy); err != nil {
		t.Fatalf("SetVerificationPolicy() error = %v", err)
	}
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
