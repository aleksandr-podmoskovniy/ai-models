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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
)

func TestServeRejectsMissingSessionStore(t *testing.T) {
	t.Parallel()

	err := Serve(t.Context(), Options{
		StagingBucket: "ai-models",
		StagingClient: &fakeStagingClient{},
	})
	if err == nil || err.Error() != "session store must not be nil" {
		t.Fatalf("expected session store validation error, got %v", err)
	}
}

func TestHandlerExposesHealthz(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: &fakeStagingClient{},
		Sessions:      &fakeSessionStore{},
	})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("unexpected status %d", got)
	}
}

func TestHandlerRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: &fakeStagingClient{},
		Sessions: &fakeSessionStore{
			sessions: map[string]SessionRecord{
				"session-a": issuedSession("session-a"),
			},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/v1/upload/session-a?token=wrong", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("unexpected status %d", got)
	}
}

func TestHandlerReturnsSessionInfo(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		PartURLTTL:    20 * time.Minute,
		StagingClient: &fakeStagingClient{
			listPartsOutput: []uploadstagingports.UploadedPart{
				{PartNumber: 1, ETag: "etag-1", SizeBytes: 64},
			},
		},
		Sessions: &fakeSessionStore{
			sessions: map[string]SessionRecord{
				"session-a": {
					SessionID:           "session-a",
					UploadTokenHash:     uploadsessiontoken.Hash("token-a"),
					ExpectedSizeBytes:   256,
					StagingKeyPrefix:    "raw/1111-2222",
					DeclaredInputFormat: modelsv1alpha1.ModelInputFormatGGUF,
					ExpiresAt:           time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC),
					Phase:               SessionPhaseUploading,
					Multipart: &SessionState{
						UploadID: "upload-1",
						Key:      "raw/1111-2222/model.gguf",
						FileName: "model.gguf",
					},
				},
			},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/v1/upload/session-a?token=token-a", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	var payload sessionInfoResponse
	decodeResponse(t, response, &payload)
	if payload.Mode != "direct-multipart-staging" {
		t.Fatalf("unexpected mode %q", payload.Mode)
	}
	if payload.DeclaredInputFormat != "GGUF" {
		t.Fatalf("unexpected declared input format %q", payload.DeclaredInputFormat)
	}
	if payload.Phase != string(SessionPhaseUploading) {
		t.Fatalf("unexpected phase %q", payload.Phase)
	}
	if payload.Multipart == nil || payload.Multipart.UploadID != "upload-1" {
		t.Fatalf("unexpected multipart payload %#v", payload.Multipart)
	}
	if len(payload.Multipart.UploadedParts) != 1 || payload.Multipart.UploadedParts[0].SizeBytes != 64 {
		t.Fatalf("unexpected uploaded parts payload %#v", payload.Multipart.UploadedParts)
	}
}

func TestHandlerProbeValidatesAndPersistsState(t *testing.T) {
	t.Parallel()

	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{"session-a": issuedSession("session-a")},
	}
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: &fakeStagingClient{},
		Sessions:      store,
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/upload/session-a/probe?token=token-a", jsonBody(t, probeUploadRequest{
		FileName: "model.gguf",
		Chunk:    []byte("GGUFpayload"),
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	session := store.sessions["session-a"]
	if session.Probe == nil || session.Probe.FileName != "model.gguf" {
		t.Fatalf("expected persisted probe state, got %#v", session)
	}
	if session.Phase != SessionPhaseProbing {
		t.Fatalf("expected probing phase, got %#v", session)
	}
}

func TestHandlerInitRequiresSuccessfulProbe(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: &fakeStagingClient{},
		Sessions: &fakeSessionStore{
			sessions: map[string]SessionRecord{"session-a": issuedSession("session-a")},
		},
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/upload/session-a/init?token=token-a", jsonBody(t, initUploadRequest{
		FileName: "model.gguf",
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusConflict; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
}

func TestHandlerInitStartsMultipartUpload(t *testing.T) {
	t.Parallel()

	client := &fakeStagingClient{
		startOutput: uploadstagingports.StartMultipartUploadOutput{UploadID: "upload-1"},
	}
	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{
			"session-a": {
				Probe: &ProbeState{
					FileName:            "model.gguf",
					ResolvedInputFormat: modelsv1alpha1.ModelInputFormatGGUF,
				},
			},
		},
	}
	store.sessions["session-a"] = mergeSession(issuedSession("session-a"), store.sessions["session-a"])
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: client,
		Sessions:      store,
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/upload/session-a/init?token=token-a", jsonBody(t, initUploadRequest{
		FileName: "model.gguf",
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if client.started.Key != "raw/1111-2222/model.gguf" {
		t.Fatalf("unexpected started multipart input %#v", client.started)
	}
	session := store.sessions["session-a"]
	if session.Multipart == nil || session.Multipart.UploadID != "upload-1" {
		t.Fatalf("expected persisted multipart state, got %#v", session)
	}
}
