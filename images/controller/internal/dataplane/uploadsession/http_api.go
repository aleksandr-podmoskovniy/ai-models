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
	"time"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func newHandler(api *sessionAPI) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/v1/upload/", api.handleUpload)
	return mux
}

func (api *sessionAPI) handleUpload(writer http.ResponseWriter, request *http.Request) {
	sessionID, pathToken, action, ok := routeFromRequestPath(request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}

	session, ok := api.loadAuthorizedSession(writer, request, sessionID, pathToken)
	if !ok {
		return
	}
	api.dispatchUploadRequest(writer, request, action, session)
}

func (api *sessionAPI) loadAuthorizedSession(writer http.ResponseWriter, request *http.Request, sessionID string, pathToken string) (SessionRecord, bool) {
	session, found, err := api.options.Sessions.Load(request.Context(), sessionID)
	switch {
	case err != nil:
		http.Error(writer, "load upload session failed", http.StatusInternalServerError)
		return SessionRecord{}, false
	case !found:
		http.NotFound(writer, request)
		return SessionRecord{}, false
	case !authorizeUploadRequest(request, pathToken, session.UploadTokenHash):
		http.Error(writer, "invalid upload token", http.StatusUnauthorized)
		return SessionRecord{}, false
	}

	if err := api.failExpiredSession(writer, request, session); err != nil {
		return SessionRecord{}, false
	}
	return session, true
}

func (api *sessionAPI) failExpiredSession(writer http.ResponseWriter, request *http.Request, session SessionRecord) error {
	if session.ExpiresAt.IsZero() || session.ExpiresAt.After(time.Now().UTC()) {
		return nil
	}
	if session.Phase == SessionPhaseExpired {
		if err := api.releaseUploadStorage(request.Context(), session); err != nil {
			http.Error(writer, "release upload storage reservation failed", http.StatusInternalServerError)
			return err
		}
		http.Error(writer, "upload session expired", http.StatusGone)
		return http.ErrAbortHandler
	}
	if !isSessionTerminal(session.Phase) {
		if err := api.options.Sessions.MarkExpired(request.Context(), session.SessionID, "upload session expired"); err != nil {
			http.Error(writer, "persist upload session expiry failed", http.StatusInternalServerError)
			return err
		}
		if err := api.releaseUploadStorage(request.Context(), session); err != nil {
			http.Error(writer, "release upload storage reservation failed", http.StatusInternalServerError)
			return err
		}
	}
	http.Error(writer, "upload session expired", http.StatusGone)
	return http.ErrAbortHandler
}

func (api *sessionAPI) dispatchUploadRequest(writer http.ResponseWriter, request *http.Request, action string, session SessionRecord) {
	switch request.Method {
	case http.MethodGet:
		api.dispatchGet(writer, request, action, session)
	case http.MethodPut:
		api.dispatchPut(writer, request, action, session)
	case http.MethodPost:
		api.dispatchPost(writer, request, action, session)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (api *sessionAPI) dispatchGet(writer http.ResponseWriter, request *http.Request, action string, session SessionRecord) {
	if action != "" {
		http.NotFound(writer, request)
		return
	}
	api.handleInfo(request.Context(), writer, session)
}

func (api *sessionAPI) dispatchPut(writer http.ResponseWriter, request *http.Request, action string, session SessionRecord) {
	if action != "" {
		http.NotFound(writer, request)
		return
	}
	api.handleDirectUpload(writer, request, session)
}

func (api *sessionAPI) dispatchPost(writer http.ResponseWriter, request *http.Request, action string, session SessionRecord) {
	switch action {
	case "/probe":
		api.handleProbe(writer, request, session)
	case "/init":
		api.handleInit(writer, request, session)
	case "/parts":
		api.handlePresignParts(writer, request, session)
	case "/complete":
		api.handleComplete(writer, request, session)
	case "/abort":
		api.handleAbort(writer, request, session)
	default:
		http.NotFound(writer, request)
	}
}

func (api *sessionAPI) handleInit(writer http.ResponseWriter, request *http.Request, session SessionRecord) {
	api.mu.Lock()
	defer api.mu.Unlock()

	logger := sessionLogger(session)

	if !canInitSession(session.Phase) {
		http.Error(writer, "upload session is already closed", http.StatusConflict)
		return
	}
	if session.Probe == nil {
		http.Error(writer, "upload session probe must succeed before init", http.StatusConflict)
		return
	}

	var body initUploadRequest
	if err := decodeJSON(request, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	fileName := sanitizedUploadFileName(body.FileName)
	if fileName == "upload.bin" {
		fileName = session.Probe.FileName
	}
	if session.Probe != nil && session.Probe.FileName != "" && session.Probe.FileName != fileName {
		http.Error(writer, "upload session is already probed for a different file", http.StatusConflict)
		return
	}
	if session.Multipart != nil {
		if session.Multipart.FileName != fileName {
			http.Error(writer, "upload session is already initialized for a different file", http.StatusConflict)
			return
		}
		logger.Info(
			"upload session multipart upload reused",
			slog.String("fileName", session.Multipart.FileName),
		)
		writeJSON(writer, http.StatusOK, initUploadResponse{
			UploadID: session.Multipart.UploadID,
			Key:      session.Multipart.Key,
			FileName: session.Multipart.FileName,
		})
		return
	}

	key, err := uploadKey(session.StagingKeyPrefix, fileName)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	started, err := api.options.StagingClient.StartMultipartUpload(request.Context(), uploadstagingports.StartMultipartUploadInput{
		Bucket: api.options.StagingBucket,
		Key:    key,
	})
	if err != nil {
		http.Error(writer, "start multipart upload failed", http.StatusInternalServerError)
		return
	}

	state := SessionState{
		UploadID: started.UploadID,
		Key:      key,
		FileName: fileName,
	}
	if err := api.options.Sessions.SaveMultipart(request.Context(), session.SessionID, state); err != nil {
		_ = api.options.StagingClient.AbortMultipartUpload(request.Context(), uploadstagingports.AbortMultipartUploadInput{
			Bucket:   api.options.StagingBucket,
			Key:      key,
			UploadID: started.UploadID,
		})
		http.Error(writer, "persist upload session state failed", http.StatusInternalServerError)
		return
	}

	logger.Info(
		"upload session multipart upload initialized",
		slog.String("fileName", state.FileName),
		slog.String("stagingBucket", strings.TrimSpace(api.options.StagingBucket)),
		slog.String("stagingKeyPrefix", strings.Trim(strings.TrimSpace(session.StagingKeyPrefix), "/")),
	)

	writeJSON(writer, http.StatusCreated, initUploadResponse{
		UploadID: state.UploadID,
		Key:      state.Key,
		FileName: state.FileName,
	})
}

func (api *sessionAPI) handlePresignParts(writer http.ResponseWriter, request *http.Request, session SessionRecord) {
	if !canMutateMultipartSession(session.Phase) {
		http.Error(writer, "upload session is already closed", http.StatusConflict)
		return
	}
	if session.Multipart == nil {
		http.Error(writer, "upload session is not initialized", http.StatusConflict)
		return
	}

	var body presignPartsRequest
	if err := decodeJSON(request, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validatePartNumbers(body.PartNumbers); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	response := presignPartsResponse{
		UploadID: session.Multipart.UploadID,
		Parts:    make([]presignedPartPayload, 0, len(body.PartNumbers)),
	}
	for _, partNumber := range body.PartNumbers {
		part, err := api.options.StagingClient.PresignUploadPart(request.Context(), uploadstagingports.PresignUploadPartInput{
			Bucket:     api.options.StagingBucket,
			Key:        session.Multipart.Key,
			UploadID:   session.Multipart.UploadID,
			PartNumber: partNumber,
			Expires:    api.options.PartURLTTL,
		})
		if err != nil {
			http.Error(writer, "presign upload part failed", http.StatusInternalServerError)
			return
		}
		response.Parts = append(response.Parts, presignedPartPayload{
			PartNumber: partNumber,
			URL:        part.URL,
		})
	}

	writeJSON(writer, http.StatusOK, response)
}
