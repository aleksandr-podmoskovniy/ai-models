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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func fetchHuggingFaceProfileSummary(
	ctx context.Context,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	selectedFiles []string,
) (*RemoteProfileSummary, error) {
	switch inputFormat {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return fetchHuggingFaceSafetensorsProfileSummary(ctx, options, repoID, resolvedRevision, selectedFiles)
	case modelsv1alpha1.ModelInputFormatGGUF:
		return fetchHuggingFaceGGUFProfileSummary(ctx, options, repoID, resolvedRevision, selectedFiles)
	default:
		return nil, nil
	}
}

func fetchHuggingFaceSafetensorsProfileSummary(
	ctx context.Context,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
) (*RemoteProfileSummary, error) {
	configPath := ""
	for _, file := range selectedFiles {
		if cleanPath, err := cleanRemoteRelativePath(file); err == nil && cleanPath == "config.json" {
			configPath = cleanPath
			break
		}
	}
	if configPath == "" {
		return nil, errors.New("huggingface safetensors summary requires config.json in selected files")
	}

	configPayload, err := fetchHuggingFaceFile(ctx, repoID, resolvedRevision, options.HFToken, configPath)
	if err != nil {
		return nil, err
	}

	weightBytes, err := sumHuggingFaceWeightBytes(ctx, repoID, resolvedRevision, options.HFToken, selectedFiles)
	if err != nil {
		return nil, err
	}
	if weightBytes <= 0 {
		return nil, errors.New("huggingface safetensors summary requires positive remote weight bytes")
	}

	return &RemoteProfileSummary{
		ConfigPayload: configPayload,
		WeightBytes:   weightBytes,
	}, nil
}

func fetchHuggingFaceGGUFProfileSummary(
	ctx context.Context,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
) (*RemoteProfileSummary, error) {
	modelFileName := ""
	for _, file := range selectedFiles {
		cleanPath, err := cleanRemoteRelativePath(file)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(strings.ToLower(cleanPath), ".gguf") {
			modelFileName = cleanPath
			break
		}
	}
	if modelFileName == "" {
		return nil, errors.New("huggingface gguf summary requires selected .gguf file")
	}

	modelSizeBytes, err := headHuggingFaceFileSize(ctx, repoID, resolvedRevision, options.HFToken, modelFileName)
	if err != nil {
		return nil, err
	}
	if modelSizeBytes <= 0 {
		return nil, errors.New("huggingface gguf summary requires positive remote model bytes")
	}

	return &RemoteProfileSummary{
		ModelFileName:  modelFileName,
		ModelSizeBytes: modelSizeBytes,
	}, nil
}

func fetchHuggingFaceFile(
	ctx context.Context,
	repoID string,
	revision string,
	token string,
	relativePath string,
) ([]byte, error) {
	sourceURL, err := (&huggingFaceHTTPSnapshotDownloader{BaseURL: huggingFaceBaseURL}).resolveURL(repoID, revision, relativePath)
	if err != nil {
		return nil, err
	}
	response, err := doGET(ctx, http.DefaultClient, sourceURL, bearerAuthHeaders(token))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, unexpectedStatusError(response, "huggingface profile summary file request")
	}
	return io.ReadAll(response.Body)
}

func sumHuggingFaceWeightBytes(
	ctx context.Context,
	repoID string,
	revision string,
	token string,
	selectedFiles []string,
) (int64, error) {
	var total int64
	for _, file := range selectedFiles {
		cleanPath, err := cleanRemoteRelativePath(file)
		if err != nil {
			return 0, err
		}
		if !strings.HasSuffix(strings.ToLower(cleanPath), ".safetensors") {
			continue
		}
		sizeBytes, err := headHuggingFaceFileSize(ctx, repoID, revision, token, cleanPath)
		if err != nil {
			return 0, err
		}
		total += sizeBytes
	}
	return total, nil
}

func headHuggingFaceFileSize(
	ctx context.Context,
	repoID string,
	revision string,
	token string,
	relativePath string,
) (int64, error) {
	sourceURL, err := (&huggingFaceHTTPSnapshotDownloader{BaseURL: huggingFaceBaseURL}).resolveURL(repoID, revision, relativePath)
	if err != nil {
		return 0, err
	}
	response, err := doHEAD(ctx, http.DefaultClient, sourceURL, bearerAuthHeaders(token))
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, unexpectedStatusError(response, "huggingface profile summary HEAD request")
	}

	contentLength := response.ContentLength
	if contentLength >= 0 {
		return contentLength, nil
	}
	rawLength := strings.TrimSpace(response.Header.Get("Content-Length"))
	if rawLength == "" {
		return 0, fmt.Errorf("huggingface profile summary HEAD response for %q missing Content-Length", relativePath)
	}
	sizeBytes, err := strconv.ParseInt(rawLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("huggingface profile summary HEAD response for %q has invalid Content-Length %q: %w", relativePath, rawLength, err)
	}
	return sizeBytes, nil
}
