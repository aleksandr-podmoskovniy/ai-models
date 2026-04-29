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
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func (api *sessionAPI) handleInfo(ctx context.Context, writer http.ResponseWriter, session SessionRecord) {
	if session.Multipart != nil && session.Phase == SessionPhaseUploading {
		updated, err := syncMultipartParts(ctx, api, session)
		if err != nil {
			http.Error(writer, "sync upload session multipart state failed", http.StatusInternalServerError)
			return
		}
		session = updated
	}
	response := sessionInfoResponse{
		Mode:                      "direct-multipart-staging",
		Phase:                     strings.TrimSpace(string(session.Phase)),
		ExpectedSizeBytes:         session.ExpectedSizeBytes,
		DeclaredInputFormat:       strings.TrimSpace(string(session.DeclaredInputFormat)),
		FailureMessage:            strings.TrimSpace(session.FailureMessage),
		PartURLTTLSeconds:         int64(api.options.PartURLTTL / time.Second),
		MinimumPartSizeBytes:      minimumMultipartPartSizeBytes,
		MaximumMultipartPartCount: maximumMultipartPartCount,
	}
	if session.Probe != nil {
		response.Probe = &probeStatePayload{
			FileName:            session.Probe.FileName,
			ResolvedInputFormat: strings.TrimSpace(string(session.Probe.ResolvedInputFormat)),
		}
	}
	response.Multipart = multipartPayload(session.Multipart)
	writeJSON(writer, http.StatusOK, response)
}

func (api *sessionAPI) handleProbe(writer http.ResponseWriter, request *http.Request, session SessionRecord) {
	api.mu.Lock()
	defer api.mu.Unlock()

	logger := sessionLogger(session)

	if !canProbeSession(session.Phase) {
		http.Error(writer, "upload session is already closed", http.StatusConflict)
		return
	}
	if session.Multipart != nil {
		http.Error(writer, "upload session is already initialized", http.StatusConflict)
		return
	}

	var body probeUploadRequest
	if err := decodeJSON(request, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	expectedSizeBytes := session.ExpectedSizeBytes
	switch {
	case body.SizeBytes < 0:
		expectedSizeBytes = body.SizeBytes
	case body.SizeBytes > 0 && expectedSizeBytes > 0 && body.SizeBytes != expectedSizeBytes:
		http.Error(writer, "upload probe sizeBytes conflicts with the active session", http.StatusConflict)
		return
	case body.SizeBytes > 0:
		expectedSizeBytes = body.SizeBytes
	}

	result, err := ingestadmission.ValidateUploadProbe(ingestadmission.UploadSession{
		Owner: ingestadmission.OwnerBinding{
			Kind:       session.OwnerKind,
			Name:       session.OwnerName,
			Namespace:  session.OwnerNamespace,
			UID:        session.OwnerUID,
			Generation: session.OwnerGeneration,
		},
		Identity:            publicationIdentity(session),
		DeclaredInputFormat: session.DeclaredInputFormat,
		ExpectedSizeBytes:   expectedSizeBytes,
	}, ingestadmission.UploadProbeInput{
		FileName: body.FileName,
		Chunk:    body.Chunk,
	})
	if err != nil {
		logger.Warn(
			"upload session probe rejected",
			slog.String("fileName", sanitizedUploadFileName(body.FileName)),
			slog.Any("error", err),
		)
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if ok := api.reserveUploadStorage(writer, request, session, expectedSizeBytes); !ok {
		return
	}
	if err := api.options.Sessions.SaveProbe(request.Context(), session.SessionID, expectedSizeBytes, ProbeState{
		FileName:            result.FileName,
		ResolvedInputFormat: result.ResolvedInputFormat,
	}); err != nil {
		_ = api.releaseUploadStorage(request.Context(), session)
		http.Error(writer, "persist upload session probe state failed", http.StatusInternalServerError)
		return
	}

	logger.Info(
		"upload session probe accepted",
		slog.String("fileName", result.FileName),
		slog.String("resolvedInputFormat", strings.TrimSpace(string(result.ResolvedInputFormat))),
	)

	writeJSON(writer, http.StatusOK, probeUploadResponse{
		FileName:            result.FileName,
		ResolvedInputFormat: strings.TrimSpace(string(result.ResolvedInputFormat)),
	})
}

func (api *sessionAPI) releaseUploadStorage(ctx context.Context, session SessionRecord) error {
	if api.options.StorageReservations == nil {
		return nil
	}
	return api.options.StorageReservations.ReleaseUpload(ctx, session)
}

func (api *sessionAPI) reserveUploadStorage(
	writer http.ResponseWriter,
	request *http.Request,
	session SessionRecord,
	expectedSizeBytes int64,
) bool {
	if api.options.StorageReservations == nil {
		return true
	}
	if expectedSizeBytes <= 0 {
		http.Error(writer, "upload sizeBytes is required when artifact storage capacity limit is enabled", http.StatusBadRequest)
		return false
	}
	err := api.options.StorageReservations.ReserveUpload(request.Context(), session, expectedSizeBytes)
	switch {
	case err == nil:
		return true
	case storagecapacity.IsInsufficientStorage(err):
		http.Error(writer, err.Error(), http.StatusInsufficientStorage)
		return false
	default:
		http.Error(writer, "reserve artifact storage capacity failed", http.StatusInternalServerError)
		return false
	}
}

func publicationIdentity(session SessionRecord) publicationdata.Identity {
	identity := publicationdata.Identity{
		Name: strings.TrimSpace(session.OwnerName),
	}
	if strings.TrimSpace(session.OwnerNamespace) != "" {
		identity.Scope = publicationdata.ScopeNamespaced
		identity.Namespace = strings.TrimSpace(session.OwnerNamespace)
		return identity
	}
	identity.Scope = publicationdata.ScopeCluster
	return identity
}
