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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testRepository = "ai-models/catalog/model"

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
	return decodeResponse[startResponse](t, postJSON(t, h.server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{
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

func decodeResponse[T any](t *testing.T, response *http.Response) T {
	t.Helper()
	var payload T
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
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
