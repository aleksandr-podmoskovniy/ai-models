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

package uploadsession

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func TestHandlerDirectUploadStagesPayloadAndMarksSessionUploaded(t *testing.T) {
	t.Parallel()

	payload := append([]byte("GGUF"), bytes.Repeat([]byte("x"), 124)...)
	client := &fakeStagingClient{
		statOutput: uploadstagingports.ObjectStat{SizeBytes: int64(len(payload))},
	}
	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{"session-a": issuedSession("session-a")},
	}
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: client,
		Sessions:      store,
	})

	request := authorizedRequest(http.MethodPut, "/v1/upload/session-a", "token-a", bytes.NewReader(payload))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if client.uploaded.Key != "raw/1111-2222/deepseek-r1.gguf" {
		t.Fatalf("unexpected upload input %#v", client.uploaded)
	}
	if !bytes.Equal(client.uploadedPayload, payload) {
		t.Fatalf("unexpected uploaded payload %q", string(client.uploadedPayload))
	}
	session := store.sessions["session-a"]
	if session.Phase != SessionPhaseUploaded {
		t.Fatalf("expected uploaded phase, got %#v", session)
	}
	if session.Probe == nil || session.Probe.FileName != "deepseek-r1.gguf" {
		t.Fatalf("expected inferred probe state, got %#v", session.Probe)
	}
	if session.StagedHandle == nil || session.StagedHandle.UploadStaging == nil {
		t.Fatalf("expected staged upload handle, got %#v", session.StagedHandle)
	}
	if session.StagedHandle.UploadStaging.SizeBytes != int64(len(payload)) {
		t.Fatalf("unexpected staged handle %#v", session.StagedHandle.UploadStaging)
	}
}

func TestHandlerDirectUploadAcceptsSecretURLToken(t *testing.T) {
	t.Parallel()

	payload := append([]byte("GGUF"), bytes.Repeat([]byte("x"), 124)...)
	client := &fakeStagingClient{
		statOutput: uploadstagingports.ObjectStat{SizeBytes: int64(len(payload))},
	}
	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{"session-a": issuedSession("session-a")},
	}
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: client,
		Sessions:      store,
	})

	request := httptest.NewRequest(http.MethodPut, "/v1/upload/session-a/token-a", bytes.NewReader(payload))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if store.sessions["session-a"].Phase != SessionPhaseUploaded {
		t.Fatalf("expected uploaded phase, got %#v", store.sessions["session-a"])
	}
}

func TestHandlerDirectUploadUsesExplicitFileName(t *testing.T) {
	t.Parallel()

	payload := append([]byte("PK\x03\x04"), bytes.Repeat([]byte("z"), 124)...)
	client := &fakeStagingClient{
		statOutput: uploadstagingports.ObjectStat{SizeBytes: int64(len(payload))},
	}
	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{"session-a": issuedSession("session-a")},
	}
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: client,
		Sessions:      store,
	})

	request := authorizedRequest(http.MethodPut, "/v1/upload/session-a?filename=bundle.zip", "token-a", bytes.NewReader(payload))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if client.uploaded.Key != "raw/1111-2222/bundle.zip" {
		t.Fatalf("unexpected upload input %#v", client.uploaded)
	}
}

func TestHandlerDirectUploadRequiresContentLengthWhenStorageReservationEnabled(t *testing.T) {
	t.Parallel()

	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{"session-a": issuedSession("session-a")},
	}
	handler := newTestHandler(Options{
		StagingBucket:       "ai-models",
		StagingClient:       &fakeStagingClient{},
		Sessions:            store,
		StorageReservations: &fakeStorageReservations{},
	})

	request := authorizedRequest(http.MethodPut, "/v1/upload/session-a", "token-a", http.NoBody)
	request.ContentLength = -1
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusLengthRequired; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if store.sessions["session-a"].Phase != SessionPhaseIssued {
		t.Fatalf("expected issued phase, got %#v", store.sessions["session-a"])
	}
}

func TestHandlerDirectUploadRejectsUnsupportedPayload(t *testing.T) {
	t.Parallel()

	payload := append([]byte("not-a-model"), bytes.Repeat([]byte("x"), 117)...)
	client := &fakeStagingClient{}
	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{"session-a": issuedSession("session-a")},
	}
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: client,
		Sessions:      store,
	})

	request := authorizedRequest(http.MethodPut, "/v1/upload/session-a", "token-a", bytes.NewReader(payload))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusBadRequest; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if client.uploaded.Body != nil {
		t.Fatalf("payload should not be staged on rejected upload")
	}
}
