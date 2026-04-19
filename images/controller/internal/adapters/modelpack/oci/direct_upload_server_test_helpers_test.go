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
	"encoding/pem"
	"net/http"
	"net/http/httptest"
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
