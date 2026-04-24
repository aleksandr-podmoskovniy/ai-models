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
	"strconv"
	"strings"
	"testing"
	"time"
)

const testRepository = "ai-models/catalog/model"

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

type serviceHarness struct {
	backend *fakeBackend
	service *Service
	server  *httptest.Server
}

func newServiceHarness(t *testing.T) *serviceHarness {
	t.Helper()

	backend := newFakeBackend()
	service, err := NewService(backend, "writer", "secret", "salt", "/dmcr", 8<<20, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	t.Cleanup(server.Close)
	return &serviceHarness{backend: backend, service: service, server: server}
}

func (h *serviceHarness) start(t *testing.T) startResponse {
	t.Helper()
	return decodeStartResponse(t, postJSON(t, h.server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{
		Repository: testRepository,
	}))
}

func (h *serviceHarness) startWithClaims(t *testing.T) (startResponse, sessionTokenClaims) {
	t.Helper()
	startPayload := h.start(t)
	claims, err := decodeSessionToken([]byte("salt"), startPayload.SessionToken)
	if err != nil {
		t.Fatalf("decodeSessionToken() error = %v", err)
	}
	return startPayload, claims
}

func (h *serviceHarness) complete(t *testing.T, request completeRequest) *http.Response {
	t.Helper()
	return postJSON(t, h.server.URL+"/v2/blob-uploads/complete", "writer", "secret", request)
}

func (h *serviceHarness) completeUpload(t *testing.T, token string, parts []UploadedPart, digest string, sizeBytes int64) *http.Response {
	t.Helper()
	return h.complete(t, completeRequest{
		SessionToken: token,
		Digest:       digest,
		SizeBytes:    sizeBytes,
		Parts:        parts,
	})
}

func standardParts() []UploadedPart {
	return []UploadedPart{
		{PartNumber: 1, ETag: "etag-1", SizeBytes: 8},
		{PartNumber: 2, ETag: "etag-2", SizeBytes: 4},
	}
}

func singlePart() []UploadedPart {
	return []UploadedPart{{PartNumber: 1, ETag: "etag-1", SizeBytes: 8}}
}

func decodeStartResponse(t *testing.T, response *http.Response) startResponse {
	t.Helper()
	var payload startResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode(startResponse) error = %v", err)
	}
	return payload
}

func decodeCompleteResponse(t *testing.T, response *http.Response) completeResponse {
	t.Helper()
	var payload completeResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode(completeResponse) error = %v", err)
	}
	return payload
}

func expectStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if got := response.StatusCode; got != want {
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
