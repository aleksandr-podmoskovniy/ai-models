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
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
)

type RemoteOptions struct {
	URL             string
	Workspace       string
	RequestedFormat modelsv1alpha1.ModelInputFormat
	HFToken         string
	HTTPCABundle    []byte
	HTTPAuthDir     string
}

type RemoteResult struct {
	SourceType        modelsv1alpha1.ModelSourceType
	ModelDir          string
	InputFormat       modelsv1alpha1.ModelInputFormat
	ExternalReference string
	ResolvedRevision  string
	Framework         string
	PipelineTag       string
	License           string
	SourceRepoID      string
}

func FetchRemoteModel(ctx context.Context, options RemoteOptions) (RemoteResult, error) {
	if strings.TrimSpace(options.URL) == "" {
		return RemoteResult{}, errors.New("remote source URL must not be empty")
	}
	if strings.TrimSpace(options.Workspace) == "" {
		return RemoteResult{}, errors.New("remote source workspace must not be empty")
	}

	sourceType, err := modelsv1alpha1.DetectRemoteSourceType(options.URL)
	if err != nil {
		return RemoteResult{}, err
	}

	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace:
		return fetchHuggingFaceModel(ctx, options)
	case modelsv1alpha1.ModelSourceTypeHTTP:
		return fetchHTTPModel(ctx, options)
	default:
		return RemoteResult{}, errors.New("unsupported remote source type")
	}
}

func fetchHuggingFaceModel(ctx context.Context, options RemoteOptions) (RemoteResult, error) {
	repoID, revision, err := modelsv1alpha1.ParseHuggingFaceURL(options.URL)
	if err != nil {
		return RemoteResult{}, err
	}

	info, err := FetchHuggingFaceInfo(ctx, repoID, revision, options.HFToken)
	if err != nil {
		return RemoteResult{}, err
	}

	inputFormat, err := resolveRemoteFormat(info.Files, options.RequestedFormat)
	if err != nil {
		return RemoteResult{}, err
	}
	selectedFiles, err := modelformat.SelectRemoteFiles(inputFormat, info.Files)
	if err != nil {
		return RemoteResult{}, err
	}

	modelDir := filepath.Join(options.Workspace, "checkpoint")
	resolvedRevision := ResolveHuggingFaceRevision(info, revision)
	if err := DownloadHuggingFaceFiles(ctx, repoID, resolvedRevision, options.HFToken, modelDir, selectedFiles); err != nil {
		return RemoteResult{}, err
	}
	if err := modelformat.ValidateDir(modelDir, inputFormat); err != nil {
		return RemoteResult{}, err
	}

	return RemoteResult{
		SourceType:        modelsv1alpha1.ModelSourceTypeHuggingFace,
		ModelDir:          modelDir,
		InputFormat:       inputFormat,
		ExternalReference: firstNonEmpty(info.ID, repoID),
		ResolvedRevision:  resolvedRevision,
		Framework:         firstNonEmpty(info.LibraryName, "transformers"),
		PipelineTag:       info.PipelineTag,
		License:           info.License,
		SourceRepoID:      info.ID,
	}, nil
}

func fetchHTTPModel(ctx context.Context, options RemoteOptions) (RemoteResult, error) {
	sourcePath, metadata, err := DownloadHTTPSource(
		ctx,
		options.URL,
		options.HTTPCABundle,
		options.HTTPAuthDir,
		filepath.Join(options.Workspace, ".download"),
	)
	if err != nil {
		return RemoteResult{}, err
	}

	modelDir, err := PrepareModelInput(sourcePath, filepath.Join(options.Workspace, "checkpoint"))
	if err != nil {
		return RemoteResult{}, err
	}

	inputFormat, err := resolveDirFormat(modelDir, options.RequestedFormat)
	if err != nil {
		return RemoteResult{}, err
	}
	if err := modelformat.ValidateDir(modelDir, inputFormat); err != nil {
		return RemoteResult{}, err
	}

	return RemoteResult{
		SourceType:        modelsv1alpha1.ModelSourceTypeHTTP,
		ModelDir:          modelDir,
		InputFormat:       inputFormat,
		ExternalReference: options.URL,
		ResolvedRevision:  metadata.ResolvedRevision(),
		Framework:         "transformers",
	}, nil
}

func resolveRemoteFormat(files []string, requested modelsv1alpha1.ModelInputFormat) (modelsv1alpha1.ModelInputFormat, error) {
	if strings.TrimSpace(string(requested)) != "" {
		return requested, nil
	}
	return modelformat.DetectRemoteFormat(files)
}

func resolveDirFormat(modelDir string, requested modelsv1alpha1.ModelInputFormat) (modelsv1alpha1.ModelInputFormat, error) {
	if strings.TrimSpace(string(requested)) != "" {
		return requested, nil
	}
	return modelformat.DetectDirFormat(modelDir)
}
