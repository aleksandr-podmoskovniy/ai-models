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

	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
)

func TestHandlerMarksExpiredSessionState(t *testing.T) {
	t.Parallel()

	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{
			"session-a": func() SessionRecord {
				session := issuedSession("session-a")
				session.ExpiresAt = time.Date(2020, 4, 10, 13, 0, 0, 0, time.UTC)
				return session
			}(),
		},
	}
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: &fakeStagingClient{},
		Sessions:      store,
	})

	request := authorizedRequest(http.MethodGet, "/v1/upload/session-a", "token-a", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusGone; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if store.sessions["session-a"].Phase != SessionPhaseExpired {
		t.Fatalf("expected expired session, got %#v", store.sessions["session-a"])
	}
}

func TestHandlerDoesNotRewritePublishingSessionToExpired(t *testing.T) {
	t.Parallel()

	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{
			"session-a": func() SessionRecord {
				session := issuedSession("session-a")
				session.Phase = SessionPhasePublishing
				session.ExpiresAt = time.Date(2020, 4, 10, 13, 0, 0, 0, time.UTC)
				return session
			}(),
		},
	}
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: &fakeStagingClient{},
		Sessions:      store,
	})

	request := authorizedRequest(http.MethodGet, "/v1/upload/session-a", "token-a", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusGone; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if store.sessions["session-a"].Phase != SessionPhasePublishing {
		t.Fatalf("publishing session phase must stay intact, got %#v", store.sessions["session-a"])
	}
}

func TestHandlerAbortMarksAbortedSession(t *testing.T) {
	t.Parallel()

	client := &fakeStagingClient{}
	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{
			"session-a": {
				SessionID:        "session-a",
				UploadTokenHash:  uploadsessiontoken.Hash("token-a"),
				StagingKeyPrefix: "raw/1111-2222",
				ExpiresAt:        time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC),
				Phase:            SessionPhaseUploading,
				Multipart: &SessionState{
					UploadID: "upload-1",
					Key:      "raw/1111-2222/model.gguf",
					FileName: "model.gguf",
				},
			},
		},
	}
	handler := newTestHandler(Options{
		StagingBucket: "ai-models",
		StagingClient: client,
		Sessions:      store,
	})

	request := authorizedRequest(http.MethodPost, "/v1/upload/session-a/abort", "token-a", http.NoBody)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusNoContent; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	session := store.sessions["session-a"]
	if session.Phase != SessionPhaseAborted {
		t.Fatalf("expected aborted session, got %#v", session)
	}
}
