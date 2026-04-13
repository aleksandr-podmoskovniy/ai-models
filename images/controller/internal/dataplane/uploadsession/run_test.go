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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
)

func TestSanitizedUploadFileName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "upload.bin"},
		{name: "basename", input: "model.tar.gz", want: "model.tar.gz"},
		{name: "path", input: "/tmp/model.gguf", want: "model.gguf"},
		{name: "windows path", input: `C:\tmp\model.gguf`, want: "model.gguf"},
		{name: "hidden", input: ".env", want: "upload.bin"},
		{name: "parent", input: "../evil.tar", want: "evil.tar"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizedUploadFileName(tc.input); got != tc.want {
				t.Fatalf("sanitizedUploadFileName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizePortDefaults(t *testing.T) {
	t.Parallel()

	if got, want := normalizePort(0), 8444; got != want {
		t.Fatalf("normalizePort(0) = %d, want %d", got, want)
	}
	if got, want := normalizePort(18080), 18080; got != want {
		t.Fatalf("normalizePort(18080) = %d, want %d", got, want)
	}
}

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

type fakeSessionStore struct {
	sessions map[string]SessionRecord
	loadErr  error
	saveErr  error
	failErr  error
}

func (s *fakeSessionStore) Load(_ context.Context, sessionID string) (SessionRecord, bool, error) {
	if s.loadErr != nil {
		return SessionRecord{}, false, s.loadErr
	}
	session, found := s.sessions[sessionID]
	return session, found, nil
}

func (s *fakeSessionStore) SaveMultipart(_ context.Context, sessionID string, state SessionState) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	session := s.sessions[sessionID]
	session.Phase = SessionPhaseUploading
	session.Multipart = &state
	s.sessions[sessionID] = session
	return nil
}

func (s *fakeSessionStore) SaveMultipartParts(_ context.Context, sessionID string, parts []UploadedPart) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	session := s.sessions[sessionID]
	if session.Multipart == nil {
		session.Multipart = &SessionState{}
	}
	session.Multipart.UploadedParts = append([]UploadedPart(nil), parts...)
	s.sessions[sessionID] = session
	return nil
}

func (s *fakeSessionStore) SaveProbe(_ context.Context, sessionID string, state ProbeState) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	session := s.sessions[sessionID]
	session.Phase = SessionPhaseProbing
	session.Probe = &state
	s.sessions[sessionID] = session
	return nil
}

func (s *fakeSessionStore) ClearMultipart(_ context.Context, sessionID string) error {
	session := s.sessions[sessionID]
	if session.Probe != nil {
		session.Phase = SessionPhaseProbing
	} else {
		session.Phase = SessionPhaseIssued
	}
	session.Multipart = nil
	s.sessions[sessionID] = session
	return nil
}

func (s *fakeSessionStore) MarkUploaded(_ context.Context, sessionID string, handle cleanuphandle.Handle) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	session := s.sessions[sessionID]
	session.Phase = SessionPhaseUploaded
	session.StagedHandle = &handle
	s.sessions[sessionID] = session
	return nil
}

func (s *fakeSessionStore) MarkFailed(_ context.Context, sessionID string, message string) error {
	if s.failErr != nil {
		return s.failErr
	}
	session := s.sessions[sessionID]
	session.Phase = SessionPhaseFailed
	session.Multipart = nil
	session.FailureMessage = message
	s.sessions[sessionID] = session
	return nil
}

func (s *fakeSessionStore) MarkAborted(_ context.Context, sessionID string, message string) error {
	if s.failErr != nil {
		return s.failErr
	}
	session := s.sessions[sessionID]
	session.Phase = SessionPhaseAborted
	session.FailureMessage = message
	s.sessions[sessionID] = session
	return nil
}

func (s *fakeSessionStore) MarkExpired(_ context.Context, sessionID string, message string) error {
	if s.failErr != nil {
		return s.failErr
	}
	session := s.sessions[sessionID]
	session.Phase = SessionPhaseExpired
	session.FailureMessage = message
	s.sessions[sessionID] = session
	return nil
}

type fakeStagingClient struct {
	started         uploadstagingports.StartMultipartUploadInput
	startOutput     uploadstagingports.StartMultipartUploadOutput
	startErr        error
	presignInputs   []uploadstagingports.PresignUploadPartInput
	presignedURL    string
	presignErr      error
	listPartsInput  uploadstagingports.ListMultipartUploadPartsInput
	listPartsOutput []uploadstagingports.UploadedPart
	listPartsErr    error
	completed       uploadstagingports.CompleteMultipartUploadInput
	completeErr     error
	aborted         uploadstagingports.AbortMultipartUploadInput
	abortErr        error
	statInput       uploadstagingports.StatInput
	statOutput      uploadstagingports.ObjectStat
	statErr         error
	deleted         uploadstagingports.DeleteInput
	deleteErr       error
}

func (c *fakeStagingClient) StartMultipartUpload(_ context.Context, input uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	c.started = input
	if c.startErr != nil {
		return uploadstagingports.StartMultipartUploadOutput{}, c.startErr
	}
	return c.startOutput, nil
}

func (c *fakeStagingClient) PresignUploadPart(_ context.Context, input uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	c.presignInputs = append(c.presignInputs, input)
	if c.presignErr != nil {
		return uploadstagingports.PresignUploadPartOutput{}, c.presignErr
	}
	return uploadstagingports.PresignUploadPartOutput{URL: c.presignedURL}, nil
}

func (c *fakeStagingClient) CompleteMultipartUpload(_ context.Context, input uploadstagingports.CompleteMultipartUploadInput) error {
	c.completed = input
	return c.completeErr
}

func (c *fakeStagingClient) ListMultipartUploadParts(_ context.Context, input uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	c.listPartsInput = input
	if c.listPartsErr != nil {
		return nil, c.listPartsErr
	}
	return append([]uploadstagingports.UploadedPart(nil), c.listPartsOutput...), nil
}

func (c *fakeStagingClient) AbortMultipartUpload(_ context.Context, input uploadstagingports.AbortMultipartUploadInput) error {
	c.aborted = input
	return c.abortErr
}

func (c *fakeStagingClient) Stat(_ context.Context, input uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	c.statInput = input
	if c.statErr != nil {
		return uploadstagingports.ObjectStat{}, c.statErr
	}
	return c.statOutput, nil
}

func (c *fakeStagingClient) Download(context.Context, uploadstagingports.DownloadInput) error {
	return nil
}

func (c *fakeStagingClient) Upload(context.Context, uploadstagingports.UploadInput) error {
	return nil
}

func (c *fakeStagingClient) Delete(_ context.Context, input uploadstagingports.DeleteInput) error {
	c.deleted = input
	return c.deleteErr
}

func jsonBody(t *testing.T, value any) *bytes.Reader {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return bytes.NewReader(payload)
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}
