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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
)

func newTestHandler(options Options) http.Handler {
	return newHandler(&sessionAPI{options: normalizeOptions(options)})
}

func issuedSession(sessionID string) SessionRecord {
	return SessionRecord{
		SessionID:           sessionID,
		UploadTokenHash:     uploadsessiontoken.Hash("token-a"),
		ExpectedSizeBytes:   128,
		StagingKeyPrefix:    "raw/1111-2222",
		DeclaredInputFormat: modelsv1alpha1.ModelInputFormatGGUF,
		OwnerUID:            "1111-2222",
		OwnerKind:           modelsv1alpha1.ModelKind,
		OwnerName:           "deepseek-r1",
		OwnerNamespace:      "team-a",
		OwnerGeneration:     3,
		ExpiresAt:           time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC),
		Phase:               SessionPhaseIssued,
	}
}

func mergeSession(base SessionRecord, overlay SessionRecord) SessionRecord {
	if overlay.Probe != nil {
		base.Probe = overlay.Probe
	}
	if overlay.Multipart != nil {
		base.Multipart = overlay.Multipart
	}
	if overlay.FailureMessage != "" {
		base.FailureMessage = overlay.FailureMessage
	}
	return base
}

func jsonBody(t testingT, value any) *bytes.Reader {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return bytes.NewReader(payload)
}

func decodeResponse(t testingT, body []byte, destination any) {
	t.Helper()
	if err := json.Unmarshal(body, destination); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func authorizedRequest(method, path, token string, body io.Reader) *http.Request {
	return httptest.NewRequest(method, secretUploadPath(path, token), body)
}

func secretUploadPath(path, token string) string {
	token = strings.TrimSpace(token)
	if token == "" || !strings.HasPrefix(path, "/v1/upload/") {
		return path
	}
	base, query, foundQuery := strings.Cut(path, "?")
	parts := strings.Split(strings.Trim(strings.TrimPrefix(base, "/v1/upload/"), "/"), "/")
	if len(parts) == 1 {
		base = "/v1/upload/" + parts[0] + "/" + token
	} else if len(parts) == 2 {
		if _, ok := uploadAction(parts[1]); ok {
			base = "/v1/upload/" + parts[0] + "/" + token + "/" + parts[1]
		}
	}
	if foundQuery {
		return base + "?" + query
	}
	return base
}

type testingT interface {
	Helper()
	Fatalf(string, ...any)
}
