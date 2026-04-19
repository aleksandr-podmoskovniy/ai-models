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

package oci

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type directUploadTestOptions struct {
	partSizeBytes int64
	failFirstPart int
}

type directUploadTestServer struct {
	server         *httptest.Server
	caFile         string
	registry       *writableRegistryServer
	partSizeBytes  int64
	failFirstPart  int
	completeCalls  int
	abortCalls     int
	listPartsCalls int
	uploadCalls    int
	sessions       map[string]*directUploadSessionState
	nextSessionID  int
	failedParts    map[string]bool
	partAttempts   map[string]int
}

type directUploadSessionState struct {
	repository string
	digest     string
	parts      map[int]uploadedDirectPart
	payloads   map[int][]byte
}

func newDirectPublishHarness(
	t *testing.T,
	options directUploadTestOptions,
) (*writableRegistryServer, *directUploadTestServer, modelpackports.RegistryAuth) {
	t.Helper()

	registry, auth := newWritableRegistryServer(t)
	t.Cleanup(registry.Close)

	directUpload := newDirectUploadTestServer(t, registry, options)
	t.Cleanup(directUpload.Close)

	return registry, directUpload, auth
}

func withDirectUploadInput(
	input modelpackports.PublishInput,
	directUpload *directUploadTestServer,
) modelpackports.PublishInput {
	input.DirectUploadEndpoint = directUpload.server.URL
	input.DirectUploadCAFile = directUpload.caFile
	input.DirectUploadInsecure = false
	return input
}

func newDirectUploadTestServer(
	t *testing.T,
	registry *writableRegistryServer,
	options directUploadTestOptions,
) *directUploadTestServer {
	t.Helper()

	server := &directUploadTestServer{
		registry:      registry,
		partSizeBytes: options.partSizeBytes,
		failFirstPart: options.failFirstPart,
		sessions:      make(map[string]*directUploadSessionState),
		failedParts:   make(map[string]bool),
		partAttempts:  make(map[string]int),
	}
	if server.partSizeBytes <= 0 {
		server.partSizeBytes = 8 << 20
	}
	server.server = httptest.NewTLSServer(http.HandlerFunc(server.serve))
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.server.Certificate().Raw})
	server.caFile = writeTempFile(t, certPEM)
	return server
}

func (s *directUploadTestServer) Close() {
	s.server.Close()
}

func (s *directUploadTestServer) partAttemptCount(partNumber int) int {
	return s.partAttempts[s.partAttemptKey("session-1", partNumber)]
}

func (s *directUploadTestServer) serve(writer http.ResponseWriter, request *http.Request) {
	switch {
	case request.URL.Path == "/v1/blob-uploads":
		s.handleStart(writer, request)
	case request.URL.Path == "/v1/blob-uploads/presign-part":
		s.handlePresignPart(writer, request)
	case request.URL.Path == "/v1/blob-uploads/parts":
		s.handleListParts(writer, request)
	case request.URL.Path == "/v1/blob-uploads/complete":
		s.handleComplete(writer, request)
	case request.URL.Path == "/v1/blob-uploads/abort":
		s.handleAbort(writer, request)
	case strings.HasPrefix(request.URL.Path, "/upload/"):
		s.handleUploadPart(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (s *directUploadTestServer) requireAPIAuth(t *testing.T, request *http.Request) {
	t.Helper()

	user, pass, ok := request.BasicAuth()
	if !ok || user != "writer" || pass != "secret" {
		t.Fatalf("unexpected direct upload auth %q/%q", user, pass)
	}
}

func (s *directUploadTestServer) handleStart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.requireAPIAuth(s.registry.state.t, request)

	var payload startDirectUploadRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.registry.state.t.Fatalf("Decode(startDirectUploadRequest) error = %v", err)
	}
	if _, ok := s.registry.state.blobs[payload.Digest]; ok {
		writeJSON(writer, startDirectUploadResponse{Complete: true})
		return
	}

	s.nextSessionID++
	sessionToken := "session-" + strconv.Itoa(s.nextSessionID)
	s.sessions[sessionToken] = &directUploadSessionState{
		repository: payload.Repository,
		digest:     payload.Digest,
		parts:      make(map[int]uploadedDirectPart),
		payloads:   make(map[int][]byte),
	}
	writeJSON(writer, startDirectUploadResponse{
		Complete:      false,
		SessionToken:  sessionToken,
		PartSizeBytes: s.partSizeBytes,
	})
}

func (s *directUploadTestServer) handlePresignPart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.requireAPIAuth(s.registry.state.t, request)

	var payload presignDirectUploadPartRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.registry.state.t.Fatalf("Decode(presignDirectUploadPartRequest) error = %v", err)
	}
	if _, ok := s.sessions[payload.SessionToken]; !ok {
		http.NotFound(writer, request)
		return
	}
	writeJSON(writer, presignDirectUploadPartResponse{
		URL: s.server.URL + "/upload/" + payload.SessionToken + "/" + strconv.Itoa(payload.PartNumber),
	})
}

func (s *directUploadTestServer) handleListParts(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.requireAPIAuth(s.registry.state.t, request)

	session := s.sessions[request.URL.Query().Get("sessionToken")]
	if session == nil {
		http.NotFound(writer, request)
		return
	}
	s.listPartsCalls++
	parts := make([]uploadedDirectPart, 0, len(session.parts))
	for _, part := range session.parts {
		parts = append(parts, part)
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	writeJSON(writer, listDirectUploadPartsResponse{Parts: parts})
}

func (s *directUploadTestServer) handleComplete(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.requireAPIAuth(s.registry.state.t, request)

	var payload completeDirectUploadRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.registry.state.t.Fatalf("Decode(completeDirectUploadRequest) error = %v", err)
	}
	session := s.sessions[payload.SessionToken]
	if session == nil {
		http.NotFound(writer, request)
		return
	}

	parts, err := normalizeUploadedDirectParts(payload.Parts)
	if err != nil {
		s.registry.state.t.Fatalf("normalizeUploadedDirectParts() error = %v", err)
	}
	var assembled []byte
	for _, part := range parts {
		assembled = append(assembled, session.payloads[part.PartNumber]...)
	}
	s.registry.state.blobs[session.digest] = assembled
	s.completeCalls++
	delete(s.sessions, payload.SessionToken)
	writer.WriteHeader(http.StatusOK)
}

func (s *directUploadTestServer) handleAbort(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.requireAPIAuth(s.registry.state.t, request)

	var payload abortDirectUploadRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.registry.state.t.Fatalf("Decode(abortDirectUploadRequest) error = %v", err)
	}
	delete(s.sessions, payload.SessionToken)
	s.abortCalls++
	writer.WriteHeader(http.StatusOK)
}

func (s *directUploadTestServer) handleUploadPart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pathParts := strings.Split(strings.TrimPrefix(request.URL.Path, "/upload/"), "/")
	if len(pathParts) != 2 {
		http.NotFound(writer, request)
		return
	}
	sessionToken := pathParts[0]
	partNumber, err := strconv.Atoi(pathParts[1])
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	session := s.sessions[sessionToken]
	if session == nil {
		http.NotFound(writer, request)
		return
	}

	attemptKey := s.partAttemptKey(sessionToken, partNumber)
	s.partAttempts[attemptKey]++
	if s.failFirstPart == partNumber && !s.failedParts[attemptKey] {
		s.failedParts[attemptKey] = true
		http.Error(writer, "transient direct upload failure", http.StatusInternalServerError)
		return
	}

	payload, err := io.ReadAll(request.Body)
	if err != nil {
		s.registry.state.t.Fatalf("ReadAll(upload part body) error = %v", err)
	}
	digestBytes := sha256.Sum256(payload)
	etag := hex.EncodeToString(digestBytes[:])
	session.payloads[partNumber] = payload
	session.parts[partNumber] = uploadedDirectPart{
		PartNumber: partNumber,
		ETag:       etag,
		SizeBytes:  int64(len(payload)),
	}
	s.uploadCalls++
	writer.Header().Set("ETag", "\""+etag+"\"")
	writer.WriteHeader(http.StatusOK)
}

func (s *directUploadTestServer) partAttemptKey(sessionToken string, partNumber int) string {
	return sessionToken + ":" + strconv.Itoa(partNumber)
}

func writeJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		panic(err)
	}
}
