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
	"context"

	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

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

func (s *fakeSessionStore) SaveProbe(_ context.Context, sessionID string, expectedSizeBytes int64, state ProbeState) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	session := s.sessions[sessionID]
	session.Phase = SessionPhaseProbing
	session.ExpectedSizeBytes = expectedSizeBytes
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
