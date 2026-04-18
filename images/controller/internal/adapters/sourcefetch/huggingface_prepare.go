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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func resolveHuggingFaceProfileSummary(
	ctx context.Context,
	logger *slog.Logger,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	selectedFiles []string,
) (*RemoteProfileSummary, error) {
	profileSummary, err := fetchHuggingFaceProfileSummaryFunc(ctx, options, repoID, resolvedRevision, inputFormat, selectedFiles)
	if err != nil {
		return nil, fmt.Errorf("huggingface remote profile summary resolution failed: %w", err)
	}
	if profileSummary != nil {
		logger.Info(
			"huggingface remote profile summary resolved",
			slog.Int64("weightBytes", profileSummary.WeightBytes),
		)
	}
	return profileSummary, nil
}

func prepareHuggingFaceSourceMirror(
	ctx context.Context,
	logger *slog.Logger,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
) (*SourceMirrorSnapshot, error) {
	sourceMirrorSnapshot, err := persistHuggingFaceMirrorManifest(ctx, options.SourceMirror, repoID, resolvedRevision, selectedFiles)
	if err != nil {
		return nil, err
	}
	if sourceMirrorSnapshot != nil {
		logger.Info("huggingface source mirror manifest persisted", slog.String("sourceMirrorPrefix", sourceMirrorSnapshot.CleanupPrefix))
	}
	return sourceMirrorSnapshot, nil
}

func prepareHuggingFacePublishSource(
	ctx context.Context,
	logger *slog.Logger,
	options RemoteOptions,
	repoID string,
	resolvedRevision string,
	selectedFiles []string,
	sourceMirrorSnapshot *SourceMirrorSnapshot,
	profileSummary *RemoteProfileSummary,
) (string, *RemoteObjectSource, error) {
	objectSource, err := buildDirectHuggingFaceObjectSource(ctx, options, logger, repoID, resolvedRevision, selectedFiles, sourceMirrorSnapshot, profileSummary)
	if err != nil {
		return "", nil, err
	}

	if sourceMirrorSnapshot != nil {
		if err := transferHuggingFaceMirrorSnapshot(ctx, logger, options, repoID, resolvedRevision, selectedFiles, sourceMirrorSnapshot); err != nil {
			return "", nil, err
		}
		logger.Info("huggingface source mirror streaming publish planned")
		return "", nil, nil
	}
	if objectSource != nil {
		logger.Info("huggingface direct remote streaming publish planned")
		return "", objectSource, nil
	}
	return "", nil, errors.New("huggingface publish source planning produced neither source mirror nor direct object source")
}
