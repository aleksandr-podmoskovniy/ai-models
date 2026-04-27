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
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func buildHuggingFaceObjectSource(
	ctx context.Context,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
) (*RemoteObjectSource, error) {
	files := make([]RemoteObjectFile, 0, len(selectedFiles))
	reader := huggingFaceHTTPObjectReader{
		httpClient: http.DefaultClient,
		token:      options.HFToken,
	}
	for _, filePath := range selectedFiles {
		metadata, err := describeHuggingFaceRemoteFile(
			ctx,
			repoID,
			resolvedRevision,
			options.HFToken,
			filePath,
			"huggingface object-source",
		)
		if err != nil {
			return nil, err
		}
		files = append(files, RemoteObjectFile{
			SourcePath: metadata.SourceURL,
			TargetPath: metadata.TargetPath,
			SizeBytes:  metadata.SizeBytes,
			ETag:       metadata.ETag,
		})
	}
	return &RemoteObjectSource{
		Reader: reader,
		Files:  files,
	}, nil
}

type huggingFaceRemoteFileMetadata struct {
	SourceURL  string
	TargetPath string
	SizeBytes  int64
	ETag       string
}

func describeHuggingFaceRemoteFile(
	ctx context.Context,
	repoID string,
	revision string,
	token string,
	relativePath string,
	requestLabel string,
) (huggingFaceRemoteFileMetadata, error) {
	cleanPath, err := cleanRemoteRelativePath(relativePath)
	if err != nil {
		return huggingFaceRemoteFileMetadata{}, err
	}
	sourceURL, err := (&huggingFaceHTTPSnapshotDownloader{BaseURL: huggingFaceBaseURL}).resolveURL(repoID, revision, cleanPath)
	if err != nil {
		return huggingFaceRemoteFileMetadata{}, err
	}
	response, err := doHEAD(ctx, http.DefaultClient, sourceURL, bearerAuthHeaders(token))
	if err != nil {
		return huggingFaceRemoteFileMetadata{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return huggingFaceRemoteFileMetadata{}, unexpectedStatusError(response, requestLabel+" HEAD request")
	}
	sizeBytes, err := responseContentLength(response)
	if err != nil {
		return huggingFaceRemoteFileMetadata{}, fmt.Errorf("%s HEAD response for %q: %w", requestLabel, cleanPath, err)
	}
	return huggingFaceRemoteFileMetadata{
		SourceURL:  sourceURL,
		TargetPath: cleanPath,
		SizeBytes:  sizeBytes,
		ETag:       strings.TrimSpace(response.Header.Get("ETag")),
	}, nil
}

func responseContentLength(response *http.Response) (int64, error) {
	if response == nil {
		return 0, fmt.Errorf("response must not be nil")
	}
	if response.ContentLength >= 0 {
		return response.ContentLength, nil
	}
	rawLength := strings.TrimSpace(response.Header.Get("Content-Length"))
	if rawLength == "" {
		return 0, fmt.Errorf("response missing Content-Length")
	}
	sizeBytes, err := strconv.ParseInt(rawLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("response has invalid Content-Length %q: %w", rawLength, err)
	}
	return sizeBytes, nil
}

func responseBodyLength(response *http.Response) (int64, error) {
	if response == nil {
		return 0, fmt.Errorf("response must not be nil")
	}
	if response.ContentLength >= 0 {
		return response.ContentLength, nil
	}
	if rawLength := strings.TrimSpace(response.Header.Get("Content-Length")); rawLength != "" {
		sizeBytes, err := strconv.ParseInt(rawLength, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("response has invalid Content-Length %q: %w", rawLength, err)
		}
		return sizeBytes, nil
	}
	if segmentLength, ok, err := contentRangeLength(response.Header.Get("Content-Range")); err != nil {
		return 0, err
	} else if ok {
		return segmentLength, nil
	}
	return 0, nil
}

func contentRangeLength(raw string) (int64, bool, error) {
	contentRange := strings.TrimSpace(raw)
	if contentRange == "" {
		return 0, false, nil
	}
	if !strings.HasPrefix(contentRange, "bytes ") {
		return 0, false, fmt.Errorf("response has invalid Content-Range %q", raw)
	}
	rangeSpec, _, found := strings.Cut(strings.TrimPrefix(contentRange, "bytes "), "/")
	if !found {
		return 0, false, fmt.Errorf("response has invalid Content-Range %q", raw)
	}
	startRaw, endRaw, found := strings.Cut(strings.TrimSpace(rangeSpec), "-")
	if !found {
		return 0, false, fmt.Errorf("response has invalid Content-Range %q", raw)
	}
	start, err := strconv.ParseInt(strings.TrimSpace(startRaw), 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("response has invalid Content-Range start %q: %w", startRaw, err)
	}
	end, err := strconv.ParseInt(strings.TrimSpace(endRaw), 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("response has invalid Content-Range end %q: %w", endRaw, err)
	}
	if end < start {
		return 0, false, fmt.Errorf("response has invalid Content-Range %q", raw)
	}
	return end - start + 1, true, nil
}

type huggingFaceHTTPObjectReader struct {
	httpClient *http.Client
	token      string
}

func (r huggingFaceHTTPObjectReader) OpenRead(ctx context.Context, sourcePath string) (RemoteOpenReadResult, error) {
	return r.openRead(ctx, strings.TrimSpace(sourcePath), 0, -1)
}

func (r huggingFaceHTTPObjectReader) OpenReadRange(ctx context.Context, sourcePath string, offset, length int64) (RemoteOpenReadResult, error) {
	return r.openRead(ctx, strings.TrimSpace(sourcePath), offset, length)
}

func (r huggingFaceHTTPObjectReader) openRead(ctx context.Context, sourcePath string, offset, length int64) (RemoteOpenReadResult, error) {
	headers := bearerAuthHeaders(r.token)
	if rangeHeader, ok := httpByteRangeHeader(offset, length); ok {
		headers["Range"] = rangeHeader
	}
	response, err := doGET(ctx, r.httpClient, sourcePath, headers)
	if err != nil {
		return RemoteOpenReadResult{}, err
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		defer response.Body.Close()
		return RemoteOpenReadResult{}, unexpectedStatusError(response, "huggingface object-source GET request")
	}
	sizeBytes, err := responseBodyLength(response)
	if err != nil {
		_ = response.Body.Close()
		return RemoteOpenReadResult{}, err
	}
	return RemoteOpenReadResult{
		Body:      response.Body,
		SizeBytes: sizeBytes,
		ETag:      strings.TrimSpace(response.Header.Get("ETag")),
	}, nil
}

func httpByteRangeHeader(offset, length int64) (string, bool) {
	if offset < 0 || length == 0 || length < -1 {
		return "", false
	}
	if offset <= 0 && length < 0 {
		return "", false
	}
	if length < 0 {
		return "bytes=" + strconv.FormatInt(offset, 10) + "-", true
	}
	return "bytes=" + strconv.FormatInt(offset, 10) + "-" + strconv.FormatInt(offset+length-1, 10), true
}
