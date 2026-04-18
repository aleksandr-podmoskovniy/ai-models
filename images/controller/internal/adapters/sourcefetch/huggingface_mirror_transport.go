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

package sourcefetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

const (
	huggingFaceMirrorPartSize       = 16 << 20
	huggingFaceMirrorUploadURLTTL   = 15 * time.Minute
	huggingFaceMirrorUploadMimeType = "application/octet-stream"
)

func mirrorHuggingFaceSnapshotFiles(
	ctx context.Context,
	options *SourceMirrorOptions,
	repoID string,
	revision string,
	token string,
	files []string,
	snapshot *SourceMirrorSnapshot,
) error {
	if options == nil || options.Store == nil || options.Client == nil || snapshot == nil {
		return errors.New("huggingface source mirror options must be fully configured")
	}
	tracker, err := loadHuggingFaceMirrorTracker(ctx, options, snapshot)
	if err != nil {
		return err
	}
	if err := tracker.setSnapshotPhase(ctx, sourcemirrorports.SnapshotPhaseDownloading); err != nil {
		return err
	}
	for _, filePath := range files {
		cleanPath, err := cleanRemoteRelativePath(filePath)
		if err != nil {
			return err
		}
		if err := mirrorHuggingFaceSnapshotFile(ctx, options, repoID, revision, token, snapshot, tracker, cleanPath); err != nil {
			_ = tracker.failFile(ctx, cleanPath, err)
			_ = tracker.setSnapshotPhaseWithError(ctx, sourcemirrorports.SnapshotPhaseFailed, err)
			return err
		}
	}
	if err := tracker.setSnapshotPhase(ctx, sourcemirrorports.SnapshotPhaseCompleted); err != nil {
		return err
	}
	snapshot.ObjectCount = int64(len(tracker.state.Files))
	snapshot.SizeBytes = tracker.totalBytesConfirmed()
	return nil
}

func mirrorHuggingFaceSnapshotFile(
	ctx context.Context,
	options *SourceMirrorOptions,
	repoID string,
	revision string,
	token string,
	snapshot *SourceMirrorSnapshot,
	tracker *huggingFaceMirrorTracker,
	relativePath string,
) error {
	objectKey := sourcemirrorports.SnapshotFileObjectKey(snapshot.CleanupPrefix, relativePath)
	stat, err := options.Client.Stat(ctx, uploadstagingports.StatInput{Bucket: options.Bucket, Key: objectKey})
	if err == nil && stat.SizeBytes >= 0 {
		return tracker.completeFile(ctx, relativePath, stat.SizeBytes)
	}

	fileState, err := tracker.ensureUpload(ctx, relativePath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(fileState.MultipartUploadID) != "" {
		if err := tracker.syncUploadedParts(ctx, options, snapshot, relativePath); err != nil {
			return err
		}
		fileState = tracker.fileState(relativePath)
	}

	sourceURL, err := (&huggingFaceHTTPSnapshotDownloader{BaseURL: huggingFaceBaseURL}).resolveURL(repoID, revision, relativePath)
	if err != nil {
		return err
	}
	response, err := rangeHuggingFaceGET(ctx, http.DefaultClient, sourceURL, bearerAuthHeaders(token), fileState.BytesConfirmed)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if fileState.BytesConfirmed > 0 && response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("huggingface mirror resume expected HTTP 206, got %d", response.StatusCode)
	}
	if fileState.BytesConfirmed == 0 && response.StatusCode != http.StatusOK {
		return unexpectedStatusError(response, "huggingface source mirror download")
	}
	completedParts, uploadedBytes, err := uploadMirrorResponse(ctx, options, tracker, relativePath, objectKey, fileState, response.Body)
	if err != nil {
		return err
	}
	if len(completedParts) == 0 {
		return errors.New("huggingface source mirror uploaded zero multipart parts")
	}
	if err := options.Client.CompleteMultipartUpload(ctx, uploadstagingports.CompleteMultipartUploadInput{
		Bucket:   options.Bucket,
		Key:      objectKey,
		UploadID: fileState.MultipartUploadID,
		Parts:    completedParts,
	}); err != nil {
		return err
	}
	return tracker.completeFile(ctx, relativePath, uploadedBytes)
}

func rangeHuggingFaceGET(
	ctx context.Context,
	httpClient *http.Client,
	rawURL string,
	headers map[string]string,
	offset int64,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if offset > 0 {
		request.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return httpClient.Do(request)
}

func uploadMirrorResponse(
	ctx context.Context,
	options *SourceMirrorOptions,
	tracker *huggingFaceMirrorTracker,
	relativePath string,
	objectKey string,
	fileState sourcemirrorports.SnapshotFileState,
	body io.Reader,
) ([]uploadstagingports.CompletedPart, int64, error) {
	completedParts := append([]uploadstagingports.CompletedPart(nil), fileState.CompletedParts...)
	uploadedBytes := fileState.BytesConfirmed
	nextPartNumber := nextMirrorPartNumber(completedParts)
	buffer := bytes.NewBuffer(make([]byte, 0, huggingFaceMirrorPartSize))
	chunk := make([]byte, 1<<20)

	for {
		readBytes, readErr := body.Read(chunk)
		if readBytes > 0 {
			_, _ = buffer.Write(chunk[:readBytes])
			for buffer.Len() >= huggingFaceMirrorPartSize {
				partPayload := append([]byte(nil), buffer.Next(huggingFaceMirrorPartSize)...)
				completed, err := uploadMirrorPart(ctx, options, objectKey, fileState.MultipartUploadID, nextPartNumber, partPayload)
				if err != nil {
					return nil, 0, err
				}
				completedParts = append(completedParts, completed)
				uploadedBytes += int64(len(partPayload))
				if err := tracker.appendCompletedPart(ctx, relativePath, completed, int64(len(partPayload))); err != nil {
					return nil, 0, err
				}
				nextPartNumber++
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, readErr
		}
	}

	if buffer.Len() > 0 {
		partPayload := append([]byte(nil), buffer.Bytes()...)
		completed, err := uploadMirrorPart(ctx, options, objectKey, fileState.MultipartUploadID, nextPartNumber, partPayload)
		if err != nil {
			return nil, 0, err
		}
		completedParts = append(completedParts, completed)
		uploadedBytes += int64(len(partPayload))
		if err := tracker.appendCompletedPart(ctx, relativePath, completed, int64(len(partPayload))); err != nil {
			return nil, 0, err
		}
	}
	return completedParts, uploadedBytes, nil
}

func uploadMirrorPart(
	ctx context.Context,
	options *SourceMirrorOptions,
	objectKey string,
	uploadID string,
	partNumber int32,
	payload []byte,
) (uploadstagingports.CompletedPart, error) {
	presigned, err := options.Client.PresignUploadPart(ctx, uploadstagingports.PresignUploadPartInput{
		Bucket:     options.Bucket,
		Key:        objectKey,
		UploadID:   uploadID,
		PartNumber: partNumber,
		Expires:    huggingFaceMirrorUploadURLTTL,
	})
	if err != nil {
		return uploadstagingports.CompletedPart{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, presigned.URL, bytes.NewReader(payload))
	if err != nil {
		return uploadstagingports.CompletedPart{}, err
	}
	request.Header.Set("Content-Type", huggingFaceMirrorUploadMimeType)
	httpClient := http.DefaultClient
	if options != nil && options.UploadHTTPClient != nil {
		httpClient = options.UploadHTTPClient
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return uploadstagingports.CompletedPart{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return uploadstagingports.CompletedPart{}, unexpectedStatusError(response, "huggingface source mirror upload part")
	}
	etag := strings.TrimSpace(response.Header.Get("ETag"))
	if etag == "" {
		return uploadstagingports.CompletedPart{}, errors.New("huggingface source mirror upload part response missing ETag")
	}
	return uploadstagingports.CompletedPart{PartNumber: partNumber, ETag: etag}, nil
}

func nextMirrorPartNumber(parts []uploadstagingports.CompletedPart) int32 {
	var maxPartNumber int32
	for _, part := range parts {
		if part.PartNumber > maxPartNumber {
			maxPartNumber = part.PartNumber
		}
	}
	return maxPartNumber + 1
}
