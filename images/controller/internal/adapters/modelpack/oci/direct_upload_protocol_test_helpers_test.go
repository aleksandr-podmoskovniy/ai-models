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
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"
)

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

	s.nextSessionID++
	sessionToken := "session-" + strconv.Itoa(s.nextSessionID)
	s.sessions[sessionToken] = &directUploadSessionState{
		repository: payload.Repository,
		parts:      make(map[int]uploadedDirectPart),
		payloads:   make(map[int][]byte),
	}
	writeJSON(writer, startDirectUploadResponse{
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
	if got, want := int64(len(assembled)), payload.SizeBytes; got != want {
		s.registry.state.t.Fatalf("complete sizeBytes = %d, want %d", got, want)
	}
	digestBytes := sha256.Sum256(assembled)
	verifiedDigest := "sha256:" + hex.EncodeToString(digestBytes[:])
	if expectedDigest := strings.TrimSpace(payload.Digest); expectedDigest != "" && expectedDigest != verifiedDigest {
		http.Error(writer, "verified digest does not match expected digest", http.StatusConflict)
		return
	}
	s.registry.state.blobs[verifiedDigest] = assembled
	s.completeCalls++
	delete(s.sessions, payload.SessionToken)
	writeJSON(writer, completeDirectUploadResponse{
		OK:        true,
		Digest:    verifiedDigest,
		SizeBytes: int64(len(assembled)),
	})
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
