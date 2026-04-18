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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
)

func TestHandlerRejectsMultipartMutationAfterControllerHandoff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		phase  SessionPhase
	}{
		{
			name:   "presign after uploaded",
			method: http.MethodPost,
			path:   "/v1/upload/session-a/parts?token=token-a",
			body: presignPartsRequest{
				PartNumbers: []int32{1},
			},
			phase: SessionPhaseUploaded,
		},
		{
			name:   "complete after publishing",
			method: http.MethodPost,
			path:   "/v1/upload/session-a/complete?token=token-a",
			body: completeUploadRequest{
				Parts: []completedPartRequest{{PartNumber: 1, ETag: "etag-1"}},
			},
			phase: SessionPhasePublishing,
		},
		{
			name:   "abort after completed",
			method: http.MethodPost,
			path:   "/v1/upload/session-a/abort?token=token-a",
			body:   nil,
			phase:  SessionPhaseCompleted,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeSessionStore{
				sessions: map[string]SessionRecord{
					"session-a": {
						SessionID:        "session-a",
						UploadTokenHash:  uploadsessiontoken.Hash("token-a"),
						StagingKeyPrefix: "raw/1111-2222",
						ExpiresAt:        time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC),
						Phase:            tc.phase,
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
				StagingClient: &fakeStagingClient{},
				Sessions:      store,
			})

			var bodyReader *bytes.Reader
			if tc.body == nil {
				bodyReader = bytes.NewReader(nil)
			} else {
				payload, err := json.Marshal(tc.body)
				if err != nil {
					t.Fatalf("Marshal() error = %v", err)
				}
				bodyReader = bytes.NewReader(payload)
			}

			request := httptest.NewRequest(tc.method, tc.path, bodyReader)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if got, want := response.Code, http.StatusConflict; got != want {
				t.Fatalf("unexpected status %d: %s", got, response.Body.String())
			}
		})
	}
}
