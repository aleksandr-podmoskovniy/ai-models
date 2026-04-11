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
	"log/slog"
	"net/http"
	"strings"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func (api *sessionAPI) handleComplete(writer http.ResponseWriter, request *http.Request, session SessionRecord) {
	api.mu.Lock()
	defer api.mu.Unlock()

	logger := sessionLogger(session)

	if !canMutateMultipartSession(session.Phase) {
		http.Error(writer, "upload session is already closed", http.StatusConflict)
		return
	}
	if session.Multipart == nil {
		http.Error(writer, "upload session is not initialized", http.StatusConflict)
		return
	}

	var body completeUploadRequest
	if err := decodeJSON(request, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	completedParts, err := normalizeCompletedParts(body.Parts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	session, err = syncMultipartParts(request.Context(), api, session)
	if err != nil {
		http.Error(writer, "sync upload session multipart state failed", http.StatusInternalServerError)
		return
	}
	if err := validateCompleteRequest(session.Multipart, completedParts); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.options.StagingClient.CompleteMultipartUpload(request.Context(), uploadstagingports.CompleteMultipartUploadInput{
		Bucket:   api.options.StagingBucket,
		Key:      session.Multipart.Key,
		UploadID: session.Multipart.UploadID,
		Parts:    completedParts,
	}); err != nil {
		http.Error(writer, "complete multipart upload failed", http.StatusInternalServerError)
		return
	}

	stat, err := api.options.StagingClient.Stat(request.Context(), uploadstagingports.StatInput{
		Bucket: api.options.StagingBucket,
		Key:    session.Multipart.Key,
	})
	if err != nil {
		http.Error(writer, "stat staged upload failed", http.StatusInternalServerError)
		return
	}
	if session.ExpectedSizeBytes > 0 && stat.SizeBytes != session.ExpectedSizeBytes {
		api.handleMismatchedSize(writer, request, session, stat.SizeBytes)
		return
	}

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    api.options.StagingBucket,
			Key:       session.Multipart.Key,
			FileName:  session.Multipart.FileName,
			SizeBytes: stat.SizeBytes,
		},
	}
	if err := api.options.Sessions.MarkUploaded(request.Context(), session.SessionID, handle); err != nil {
		http.Error(writer, "persist upload completion failed", http.StatusInternalServerError)
		return
	}

	logger.Info(
		"upload session raw stage completed",
		slog.String("fileName", session.Multipart.FileName),
		slog.String("stagingBucket", strings.TrimSpace(api.options.StagingBucket)),
		slog.Int64("stagedSizeBytes", stat.SizeBytes),
		slog.Int("uploadedPartCount", len(session.Multipart.UploadedParts)),
	)

	writer.WriteHeader(http.StatusCreated)
	_, _ = writer.Write([]byte("upload staged\n"))
}

func (api *sessionAPI) handleAbort(writer http.ResponseWriter, request *http.Request, session SessionRecord) {
	api.mu.Lock()
	defer api.mu.Unlock()

	logger := sessionLogger(session)

	if !canInitSession(session.Phase) {
		http.Error(writer, "upload session is already closed", http.StatusConflict)
		return
	}
	if session.Multipart == nil {
		if err := api.options.Sessions.MarkAborted(request.Context(), session.SessionID, "upload session aborted"); err != nil {
			http.Error(writer, "persist upload abort failed", http.StatusInternalServerError)
			return
		}
		logger.Info("upload session aborted before multipart initialization")
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	if err := api.options.StagingClient.AbortMultipartUpload(request.Context(), uploadstagingports.AbortMultipartUploadInput{
		Bucket:   api.options.StagingBucket,
		Key:      session.Multipart.Key,
		UploadID: session.Multipart.UploadID,
	}); err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		http.Error(writer, "abort multipart upload failed", http.StatusInternalServerError)
		return
	}
	if err := api.options.Sessions.MarkAborted(request.Context(), session.SessionID, "upload session aborted"); err != nil {
		http.Error(writer, "persist upload abort failed", http.StatusInternalServerError)
		return
	}
	logger.Info(
		"upload session aborted",
		slog.String("fileName", session.Multipart.FileName),
	)
	writer.WriteHeader(http.StatusNoContent)
}
