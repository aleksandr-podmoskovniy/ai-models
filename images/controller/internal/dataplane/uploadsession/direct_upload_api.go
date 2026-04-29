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
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

const directUploadContentType = "application/octet-stream"

func (api *sessionAPI) handleDirectUpload(writer http.ResponseWriter, request *http.Request, session SessionRecord) {
	api.mu.Lock()
	defer api.mu.Unlock()
	defer request.Body.Close()

	logger := sessionLogger(session)

	if !canProbeSession(session.Phase) {
		http.Error(writer, "upload session is already closed", http.StatusConflict)
		return
	}
	if session.Multipart != nil {
		http.Error(writer, "upload session multipart upload is already initialized", http.StatusConflict)
		return
	}

	expectedSizeBytes, ok := directUploadExpectedSize(writer, request, session, api.options.StorageReservations != nil)
	if !ok {
		return
	}
	chunk, body, err := directUploadBody(request.Body)
	if err != nil {
		http.Error(writer, "read upload payload failed", http.StatusBadRequest)
		return
	}

	fileName := directUploadFileName(request, session, chunk)
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
		FileName: fileName,
		Chunk:    chunk,
	})
	if err != nil {
		logger.Warn(
			"upload session direct upload rejected",
			slog.String("fileName", sanitizedUploadFileName(fileName)),
			slog.Any("error", err),
		)
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if ok := api.reserveUploadStorage(writer, request, session, expectedSizeBytes); !ok {
		return
	}

	key, err := uploadKey(session.StagingKeyPrefix, result.FileName)
	if err != nil {
		_ = api.releaseUploadStorage(request.Context(), session)
		http.Error(writer, err.Error(), http.StatusBadRequest)
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

	if err := api.options.StagingClient.Upload(request.Context(), uploadstagingports.UploadInput{
		Bucket:      api.options.StagingBucket,
		Key:         key,
		Body:        body,
		ContentType: directUploadRequestContentType(request),
	}); err != nil {
		_ = api.releaseUploadStorage(request.Context(), session)
		http.Error(writer, "stage direct upload payload failed", http.StatusInternalServerError)
		return
	}

	stat, err := api.options.StagingClient.Stat(request.Context(), uploadstagingports.StatInput{
		Bucket: api.options.StagingBucket,
		Key:    key,
	})
	if err != nil {
		_ = api.releaseUploadStorage(request.Context(), session)
		http.Error(writer, "stat staged upload failed", http.StatusInternalServerError)
		return
	}
	if expectedSizeBytes > 0 && stat.SizeBytes != expectedSizeBytes {
		api.handleStagedUploadSizeMismatch(writer, request, session, key, stat.SizeBytes)
		return
	}

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    api.options.StagingBucket,
			Key:       key,
			FileName:  result.FileName,
			SizeBytes: stat.SizeBytes,
		},
	}
	if err := api.options.Sessions.MarkUploaded(request.Context(), session.SessionID, handle); err != nil {
		_ = api.releaseUploadStorage(request.Context(), session)
		http.Error(writer, "persist upload completion failed", http.StatusInternalServerError)
		return
	}

	logger.Info(
		"upload session direct stage completed",
		slog.String("fileName", result.FileName),
		slog.String("stagingBucket", strings.TrimSpace(api.options.StagingBucket)),
		slog.String("stagingKey", key),
		slog.Int64("stagedSizeBytes", stat.SizeBytes),
	)

	writer.WriteHeader(http.StatusCreated)
	_, _ = writer.Write([]byte("upload staged\n"))
}

func directUploadExpectedSize(
	writer http.ResponseWriter,
	request *http.Request,
	session SessionRecord,
	requireContentLength bool,
) (int64, bool) {
	contentLength := request.ContentLength
	if requireContentLength && contentLength <= 0 {
		http.Error(writer, "upload Content-Length is required when artifact storage capacity limit is enabled", http.StatusLengthRequired)
		return 0, false
	}

	expectedSizeBytes := session.ExpectedSizeBytes
	switch {
	case contentLength < 0:
		return expectedSizeBytes, true
	case expectedSizeBytes > 0 && contentLength > 0 && contentLength != expectedSizeBytes:
		http.Error(writer, "upload Content-Length conflicts with the active session expected size", http.StatusConflict)
		return 0, false
	case contentLength > 0:
		return contentLength, true
	default:
		return expectedSizeBytes, true
	}
}

func directUploadBody(body io.Reader) ([]byte, io.Reader, error) {
	var probe bytes.Buffer
	_, err := io.CopyN(&probe, body, ingestadmission.MaxUploadProbeBytes)
	switch {
	case err == nil:
	case errors.Is(err, io.EOF):
		err = nil
	default:
		return nil, nil, err
	}

	chunk := probe.Bytes()
	return chunk, io.MultiReader(bytes.NewReader(chunk), body), nil
}

func directUploadFileName(request *http.Request, session SessionRecord, chunk []byte) string {
	candidates := []string{
		request.URL.Query().Get("filename"),
		request.Header.Get("X-Upload-Filename"),
		contentDispositionFileName(request.Header.Get("Content-Disposition")),
	}
	for _, candidate := range candidates {
		if fileName := sanitizedUploadFileName(candidate); fileName != "upload.bin" {
			return fileName
		}
	}
	return inferredDirectUploadFileName(session.OwnerName, chunk)
}

func contentDispositionFileName(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(raw)
	if err != nil {
		return ""
	}
	return params["filename"]
}

func inferredDirectUploadFileName(ownerName string, chunk []byte) string {
	base := strings.TrimSuffix(sanitizedUploadFileName(ownerName), ".bin")
	if base == "" || base == "upload" {
		base = "model"
	}
	switch {
	case hasGGUFMagic(chunk):
		return base + ".gguf"
	case looksLikeZIP(chunk):
		return base + ".zip"
	case looksLikeGzip(chunk):
		return base + ".tar.gz"
	case looksLikeZstd(chunk):
		return base + ".tar.zst"
	case looksLikeTar(chunk):
		return base + ".tar"
	default:
		return base + ".bin"
	}
}

func directUploadRequestContentType(request *http.Request) string {
	contentType := strings.TrimSpace(request.Header.Get("Content-Type"))
	if contentType == "" {
		return directUploadContentType
	}
	return contentType
}

func hasGGUFMagic(chunk []byte) bool {
	return len(chunk) >= 4 && string(chunk[:4]) == "GGUF"
}

func looksLikeZIP(chunk []byte) bool {
	return len(chunk) >= 4 && (bytes.Equal(chunk[:4], []byte("PK\x03\x04")) ||
		bytes.Equal(chunk[:4], []byte("PK\x05\x06")) ||
		bytes.Equal(chunk[:4], []byte("PK\x07\x08")))
}

func looksLikeGzip(chunk []byte) bool {
	return len(chunk) >= 2 && chunk[0] == 0x1f && chunk[1] == 0x8b
}

func looksLikeZstd(chunk []byte) bool {
	return len(chunk) >= 4 && chunk[0] == 0x28 && chunk[1] == 0xB5 && chunk[2] == 0x2F && chunk[3] == 0xFD
}

func looksLikeTar(chunk []byte) bool {
	return len(chunk) >= 265 && string(chunk[257:262]) == "ustar"
}
