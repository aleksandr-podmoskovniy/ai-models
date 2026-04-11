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
	"strings"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func isSessionTerminal(phase SessionPhase) bool {
	switch phase {
	case SessionPhaseUploaded, SessionPhasePublishing, SessionPhaseCompleted, SessionPhaseFailed, SessionPhaseAborted, SessionPhaseExpired:
		return true
	default:
		return false
	}
}

func canProbeSession(phase SessionPhase) bool {
	switch phase {
	case SessionPhaseIssued, SessionPhaseProbing:
		return true
	default:
		return false
	}
}

func canInitSession(phase SessionPhase) bool {
	switch phase {
	case SessionPhaseIssued, SessionPhaseProbing, SessionPhaseUploading:
		return true
	default:
		return false
	}
}

func canMutateMultipartSession(phase SessionPhase) bool {
	return phase == SessionPhaseUploading
}

func syncMultipartParts(ctx context.Context, api *sessionAPI, session SessionRecord) (SessionRecord, error) {
	if api == nil || session.Multipart == nil {
		return session, nil
	}
	parts, err := api.options.StagingClient.ListMultipartUploadParts(ctx, uploadstagingports.ListMultipartUploadPartsInput{
		Bucket:   strings.TrimSpace(api.options.StagingBucket),
		Key:      strings.TrimSpace(session.Multipart.Key),
		UploadID: strings.TrimSpace(session.Multipart.UploadID),
	})
	if err != nil {
		return session, err
	}
	state := *session.Multipart
	state.UploadedParts = uploadedPartsFromStaging(parts)
	if err := api.options.Sessions.SaveMultipartParts(ctx, session.SessionID, state.UploadedParts); err != nil {
		return session, err
	}
	session.Multipart = &state
	return session, nil
}

func uploadedPartsFromStaging(parts []uploadstagingports.UploadedPart) []UploadedPart {
	if len(parts) == 0 {
		return nil
	}
	result := make([]UploadedPart, 0, len(parts))
	for _, part := range parts {
		result = append(result, UploadedPart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(part.ETag),
			SizeBytes:  part.SizeBytes,
		})
	}
	return result
}

func multipartPayload(state *SessionState) *multipartStatePayload {
	if state == nil {
		return nil
	}
	payload := &multipartStatePayload{
		UploadID: state.UploadID,
		Key:      state.Key,
		FileName: state.FileName,
	}
	if len(state.UploadedParts) > 0 {
		payload.UploadedParts = make([]uploadedPartPayload, 0, len(state.UploadedParts))
		for _, part := range state.UploadedParts {
			payload.UploadedParts = append(payload.UploadedParts, uploadedPartPayload{
				PartNumber: part.PartNumber,
				ETag:       part.ETag,
				SizeBytes:  part.SizeBytes,
			})
		}
	}
	return payload
}
