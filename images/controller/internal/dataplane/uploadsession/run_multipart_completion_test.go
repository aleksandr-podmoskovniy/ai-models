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

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
)

func TestHandlerPresignsUploadParts(t *testing.T) {
	t.Parallel()

	client := &fakeStagingClient{
		presignedURL: "https://s3.example.com/upload-part",
	}
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
		PartURLTTL:    10 * time.Minute,
		StagingClient: client,
		Sessions:      store,
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/upload/session-a/parts?token=token-a", jsonBody(t, presignPartsRequest{
		PartNumbers: []int32{1, 2},
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if len(client.presignInputs) != 2 {
		t.Fatalf("expected 2 presign calls, got %d", len(client.presignInputs))
	}
}

func TestHandlerCompleteMarksUploadedSession(t *testing.T) {
	t.Parallel()

	client := &fakeStagingClient{
		listPartsOutput: []uploadstagingports.UploadedPart{
			{PartNumber: 1, ETag: "etag-1", SizeBytes: 128},
		},
		statOutput: uploadstagingports.ObjectStat{SizeBytes: 128},
	}
	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{
			"session-a": {
				SessionID:         "session-a",
				UploadTokenHash:   uploadsessiontoken.Hash("token-a"),
				ExpectedSizeBytes: 128,
				StagingKeyPrefix:  "raw/1111-2222",
				ExpiresAt:         time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC),
				Phase:             SessionPhaseUploading,
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

	request := httptest.NewRequest(http.MethodPost, "/v1/upload/session-a/complete?token=token-a", jsonBody(t, completeUploadRequest{
		Parts: []completedPartRequest{{PartNumber: 1, ETag: "etag-1"}},
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	session := store.sessions["session-a"]
	if session.Phase != SessionPhaseUploaded || session.StagedHandle == nil {
		t.Fatalf("expected uploaded session, got %#v", session)
	}
	if session.Multipart == nil || len(session.Multipart.UploadedParts) != 1 {
		t.Fatalf("expected persisted multipart manifest, got %#v", session.Multipart)
	}
}

func TestHandlerCompleteMarksFailedOnSizeMismatch(t *testing.T) {
	t.Parallel()

	client := &fakeStagingClient{
		listPartsOutput: []uploadstagingports.UploadedPart{
			{PartNumber: 1, ETag: "etag-1", SizeBytes: 64},
		},
		statOutput: uploadstagingports.ObjectStat{SizeBytes: 64},
	}
	store := &fakeSessionStore{
		sessions: map[string]SessionRecord{
			"session-a": {
				SessionID:         "session-a",
				UploadTokenHash:   uploadsessiontoken.Hash("token-a"),
				ExpectedSizeBytes: 128,
				StagingKeyPrefix:  "raw/1111-2222",
				ExpiresAt:         time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC),
				Phase:             SessionPhaseUploading,
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

	request := httptest.NewRequest(http.MethodPost, "/v1/upload/session-a/complete?token=token-a", jsonBody(t, completeUploadRequest{
		Parts: []completedPartRequest{{PartNumber: 1, ETag: "etag-1"}},
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusBadRequest; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	session := store.sessions["session-a"]
	if session.Phase != SessionPhaseFailed {
		t.Fatalf("expected failed session, got %#v", session)
	}
}
