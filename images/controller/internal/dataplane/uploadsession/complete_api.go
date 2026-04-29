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

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func (api *sessionAPI) handleMismatchedSize(writer http.ResponseWriter, request *http.Request, session SessionRecord, actualSizeBytes int64) {
	if session.Multipart == nil {
		http.Error(writer, "uploaded payload size mismatch and missing staged object state", http.StatusInternalServerError)
		return
	}
	api.handleStagedUploadSizeMismatch(writer, request, session, session.Multipart.Key, actualSizeBytes)
}

func (api *sessionAPI) handleStagedUploadSizeMismatch(
	writer http.ResponseWriter,
	request *http.Request,
	session SessionRecord,
	stagingKey string,
	actualSizeBytes int64,
) {
	logger := sessionLogger(session)
	deleteErr := api.options.StagingClient.Delete(request.Context(), uploadstagingports.DeleteInput{
		Bucket: api.options.StagingBucket,
		Key:    stagingKey,
	})
	failErr := api.options.Sessions.MarkFailed(
		request.Context(),
		session.SessionID,
		"uploaded payload size does not match expected-size-bytes",
	)
	switch {
	case deleteErr != nil:
		http.Error(writer, "uploaded payload size mismatch and cleanup failed", http.StatusInternalServerError)
	case failErr != nil:
		http.Error(writer, "uploaded payload size mismatch and state update failed", http.StatusInternalServerError)
	default:
		if err := api.releaseUploadStorage(request.Context(), session); err != nil {
			http.Error(writer, "release upload storage reservation failed", http.StatusInternalServerError)
			return
		}
		logger.Warn(
			"upload session uploaded payload size mismatch",
			slog.Int64("expectedSizeBytes", session.ExpectedSizeBytes),
			slog.Int64("actualSizeBytes", actualSizeBytes),
		)
		http.Error(writer, "uploaded payload size does not match expected-size-bytes", http.StatusBadRequest)
	}
}
