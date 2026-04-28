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
	"log/slog"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	"github.com/deckhouse/ai-models/controller/internal/domain/modelsource"
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
)

var (
	huggingFaceBaseURL                 = "https://huggingface.co"
	fetchHuggingFaceInfoFunc           = FetchHuggingFaceInfo
	fetchHuggingFaceProfileSummaryFunc = fetchHuggingFaceProfileSummary
)

type HuggingFaceInfo struct {
	ID           string
	SHA          string
	PipelineTag  string
	DeclaredTask string
	License      string
	Files        []string
}

type huggingFaceAPIResponse struct {
	ID          string                   `json:"id"`
	SHA         string                   `json:"sha"`
	PipelineTag string                   `json:"pipeline_tag"`
	CardData    huggingFaceCardData      `json:"cardData"`
	Siblings    []huggingFaceSiblingInfo `json:"siblings"`
}

type huggingFaceSiblingInfo struct {
	Path string `json:"rfilename"`
}

type huggingFaceCardData struct {
	License    string                  `json:"license"`
	ModelIndex []huggingFaceModelIndex `json:"model-index"`
}

type huggingFaceModelIndex struct {
	Results []huggingFaceModelResult `json:"results"`
}

type huggingFaceModelResult struct {
	Task huggingFaceModelTask `json:"task"`
}

type huggingFaceModelTask struct {
	Type string `json:"type"`
}

func fetchHuggingFaceModel(ctx context.Context, options RemoteOptions) (RemoteResult, error) {
	repoID, revision, err := modelsource.ParseHuggingFaceURL(options.URL)
	if err != nil {
		return RemoteResult{}, err
	}

	logger := slog.Default().With(
		slog.String("sourceType", string(modelsv1alpha1.ModelSourceTypeHuggingFace)),
		slog.String("sourceRepoID", repoID),
		slog.String("requestedRevision", revision),
	)

	info, err := fetchHuggingFaceMetadata(ctx, logger, repoID, revision, options.HFToken)
	if err != nil {
		return RemoteResult{}, err
	}

	inputFormat, selectedFiles, err := resolveHuggingFaceSelection(logger, options, info)
	if err != nil {
		return RemoteResult{}, err
	}

	resolvedRevision := ResolveHuggingFaceRevision(info, revision)
	logger.Debug("huggingface resolved revision selected", slog.String("resolvedRevision", resolvedRevision))

	profileSummary, err := resolveHuggingFaceProfileSummary(ctx, logger, options, repoID, resolvedRevision, inputFormat, selectedFiles)
	if err != nil {
		return RemoteResult{}, err
	}

	sourceMirrorSnapshot, err := prepareHuggingFaceSourceMirror(ctx, logger, options, repoID, resolvedRevision, selectedFiles)
	if err != nil {
		return RemoteResult{}, err
	}

	modelDir, objectSource, err := prepareHuggingFacePublishSource(
		ctx,
		logger,
		options,
		repoID,
		resolvedRevision,
		selectedFiles,
		sourceMirrorSnapshot,
		profileSummary,
	)
	if err != nil {
		return RemoteResult{}, err
	}

	return RemoteResult{
		SourceType:     modelsv1alpha1.ModelSourceTypeHuggingFace,
		ModelDir:       modelDir,
		InputFormat:    inputFormat,
		SelectedFiles:  append([]string(nil), selectedFiles...),
		ObjectSource:   objectSource,
		ProfileSummary: profileSummary,
		Provenance: RemoteProvenance{
			ExternalReference: firstNonEmpty(info.ID, repoID),
			ResolvedRevision:  resolvedRevision,
		},
		Fallbacks: RemoteProfileFallbacks{
			SourceDeclaredTask: firstNonEmpty(info.DeclaredTask, info.PipelineTag),
		},
		Metadata: RemoteMetadata{
			License:      info.License,
			SourceRepoID: info.ID,
		},
		SourceMirror: sourceMirrorSnapshot,
	}, nil
}

func buildDirectHuggingFaceObjectSource(
	ctx context.Context,
	options RemoteOptions,
	logger *slog.Logger,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
	sourceMirrorSnapshot *SourceMirrorSnapshot,
	profileSummary *RemoteProfileSummary,
) (*RemoteObjectSource, error) {
	if sourceMirrorSnapshot != nil {
		return nil, nil
	}
	if !options.SkipLocalMaterialization {
		return nil, errors.New("huggingface remote publication no longer supports local materialization fallback")
	}
	if profileSummary == nil {
		return nil, errors.New("huggingface direct object-source publish requires remote profile summary")
	}

	objectSource, err := buildHuggingFaceObjectSource(ctx, options, repoID, resolvedRevision, selectedFiles)
	if err != nil {
		return nil, fmt.Errorf("huggingface direct object-source planning failed: %w", err)
	}
	logger.Info("huggingface direct object-source publish planned", slog.Int("selectedFileCount", len(objectSource.Files)))
	return objectSource, nil
}

func fetchHuggingFaceMetadata(
	ctx context.Context,
	logger *slog.Logger,
	repoID string,
	revision string,
	token string,
) (HuggingFaceInfo, error) {
	started := time.Now()
	logger.Info("huggingface metadata request started")
	info, err := fetchHuggingFaceInfoFunc(ctx, repoID, revision, token)
	if err != nil {
		return HuggingFaceInfo{}, err
	}
	logger.Info(
		"huggingface metadata request completed",
		slog.Int64("durationMs", time.Since(started).Milliseconds()),
		slog.String("resolvedRepoID", info.ID),
		slog.String("resolvedSHA", info.SHA),
		slog.String("pipelineTag", info.PipelineTag),
		slog.Int("remoteFileCount", len(info.Files)),
	)
	return info, nil
}

func resolveHuggingFaceSelection(
	logger *slog.Logger,
	options RemoteOptions,
	info HuggingFaceInfo,
) (modelsv1alpha1.ModelInputFormat, []string, error) {
	inputFormat, err := resolveRemoteFormat(info.Files, options.RequestedFormat)
	if err != nil {
		return "", nil, err
	}
	selectedFiles, err := modelformat.SelectRemoteFiles(inputFormat, info.Files)
	if err != nil {
		return "", nil, err
	}
	logger.Info(
		"huggingface source files selected",
		slog.String("resolvedInputFormat", string(inputFormat)),
		slog.Int("selectedFileCount", len(selectedFiles)),
		slog.Bool("sourceMirrorEnabled", options.SourceMirror != nil),
	)
	if len(selectedFiles) > 0 {
		logger.Debug("huggingface selected file sample", slog.Any("selectedFilesSample", sampleRemoteFiles(selectedFiles, 8)))
	}
	return inputFormat, selectedFiles, nil
}

func transferHuggingFaceMirrorSnapshot(
	ctx context.Context,
	logger *slog.Logger,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
	sourceMirrorSnapshot *SourceMirrorSnapshot,
) error {
	mirrorStarted := time.Now()
	logger.Info("huggingface source mirror transfer started", slog.Int("selectedFileCount", len(selectedFiles)))
	if err := mirrorHuggingFaceSnapshotFiles(ctx, options.SourceMirror, repoID, resolvedRevision, options.HFToken, selectedFiles, sourceMirrorSnapshot); err != nil {
		_ = persistHuggingFaceMirrorPhase(ctx, options.SourceMirror, sourceMirrorSnapshot, sourcemirrorports.SnapshotPhaseFailed, nil, err.Error())
		return err
	}
	logger.Info(
		"huggingface source mirror transfer completed",
		slog.Int64("durationMs", time.Since(mirrorStarted).Milliseconds()),
		slog.Int64("sourceMirrorObjectCount", sourceMirrorSnapshot.ObjectCount),
		slog.Int64("sourceMirrorSizeBytes", sourceMirrorSnapshot.SizeBytes),
	)
	return nil
}

func sampleRemoteFiles(files []string, limit int) []string {
	if limit <= 0 || len(files) <= limit {
		return append([]string(nil), files...)
	}
	sample := append([]string(nil), files[:limit]...)
	return append(sample, "...")
}
