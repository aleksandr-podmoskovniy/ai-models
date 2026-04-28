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
	"io"
	"net/http"
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
	case modelsv1alpha1.ModelInputFormatDiffusers:
		return fetchHuggingFaceDiffusersProfileSummary(ctx, options, repoID, resolvedRevision, selectedFiles)
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return fetchHuggingFaceSafetensorsProfileSummary(ctx, options, repoID, resolvedRevision, selectedFiles)
	case modelsv1alpha1.ModelInputFormatGGUF:
		return fetchHuggingFaceGGUFProfileSummary(ctx, options, repoID, resolvedRevision, selectedFiles)
	default:
		return nil, nil
	}
}

func fetchHuggingFaceDiffusersProfileSummary(
	ctx context.Context,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
) (*RemoteProfileSummary, error) {
	modelIndexPayload, err := fetchHuggingFaceFile(ctx, repoID, resolvedRevision, options.HFToken, "model_index.json")
	if err != nil {
		return nil, err
	}
	weightStats, err := sumHuggingFaceWeightStats(ctx, repoID, resolvedRevision, options.HFToken, selectedFiles)
	if err != nil {
		return nil, err
	}
	if weightStats.TotalBytes <= 0 {
		return nil, errors.New("huggingface diffusers summary requires positive remote weight bytes")
	}
	return &RemoteProfileSummary{
		ModelIndexPayload:      modelIndexPayload,
		WeightBytes:            weightStats.TotalBytes,
		LargestWeightFileBytes: weightStats.LargestFileBytes,
		WeightFileCount:        weightStats.FileCount,
	}, nil
}

func fetchHuggingFaceSafetensorsProfileSummary(
	ctx context.Context,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
) (*RemoteProfileSummary, error) {
	configPath := huggingFaceSafetensorsProfileConfigPath(selectedFiles)
	if configPath == "" {
		return nil, errors.New("huggingface safetensors summary requires config.json or model_index.json in selected files")
	}

	configPayload, err := fetchHuggingFaceFile(ctx, repoID, resolvedRevision, options.HFToken, configPath)
	if err != nil {
		return nil, err
	}
	tokenizerConfigPayload, err := fetchOptionalHuggingFaceProfileFile(ctx, options, repoID, resolvedRevision, selectedFiles, "tokenizer_config.json")
	if err != nil {
		return nil, err
	}

	weightStats, err := sumHuggingFaceWeightStats(ctx, repoID, resolvedRevision, options.HFToken, selectedFiles)
	if err != nil {
		return nil, err
	}
	if weightStats.TotalBytes <= 0 {
		return nil, errors.New("huggingface safetensors summary requires positive remote weight bytes")
	}

	return &RemoteProfileSummary{
		ConfigPayload:          configPayload,
		TokenizerConfigPayload: tokenizerConfigPayload,
		WeightBytes:            weightStats.TotalBytes,
		LargestWeightFileBytes: weightStats.LargestFileBytes,
		WeightFileCount:        weightStats.FileCount,
	}, nil
}

func huggingFaceSafetensorsProfileConfigPath(selectedFiles []string) string {
	fallbackPath := ""
	for _, file := range selectedFiles {
		cleanPath, err := cleanRemoteRelativePath(file)
		if err != nil {
			continue
		}
		switch cleanPath {
		case "config.json":
			return cleanPath
		case "model_index.json":
			fallbackPath = cleanPath
		}
	}
	return fallbackPath
}

func fetchOptionalHuggingFaceProfileFile(
	ctx context.Context,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
	relativePath string,
) ([]byte, error) {
	for _, file := range selectedFiles {
		if cleanPath, err := cleanRemoteRelativePath(file); err == nil && cleanPath == relativePath {
			return fetchHuggingFaceFile(ctx, repoID, resolvedRevision, options.HFToken, cleanPath)
		}
	}
	return nil, nil
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

	metadata, err := describeHuggingFaceRemoteFile(
		ctx,
		repoID,
		resolvedRevision,
		options.HFToken,
		modelFileName,
		"huggingface profile summary",
	)
	if err != nil {
		return nil, err
	}
	if metadata.SizeBytes <= 0 {
		return nil, errors.New("huggingface gguf summary requires positive remote model bytes")
	}

	return &RemoteProfileSummary{
		ModelFileName:  modelFileName,
		ModelSizeBytes: metadata.SizeBytes,
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

func sumHuggingFaceWeightStats(
	ctx context.Context,
	repoID string,
	revision string,
	token string,
	selectedFiles []string,
) (WeightStats, error) {
	var stats WeightStats
	for _, file := range selectedFiles {
		cleanPath, err := cleanRemoteRelativePath(file)
		if err != nil {
			return WeightStats{}, err
		}
		if !strings.HasSuffix(strings.ToLower(cleanPath), ".safetensors") {
			continue
		}
		metadata, err := describeHuggingFaceRemoteFile(
			ctx,
			repoID,
			revision,
			token,
			cleanPath,
			"huggingface profile summary",
		)
		if err != nil {
			return WeightStats{}, err
		}
		stats.add(metadata.SizeBytes)
	}
	return stats, nil
}
