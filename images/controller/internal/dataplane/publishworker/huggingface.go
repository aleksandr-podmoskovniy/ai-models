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

package publishworker

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func publishFromHuggingFace(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.HFModelID) == "" {
		return publicationartifact.Result{}, errors.New("hf-model-id is required")
	}

	logger := slog.Default().With(
		slog.String("sourceType", string(modelsv1alpha1.ModelSourceTypeHuggingFace)),
		slog.String("sourceRepoID", strings.TrimSpace(options.HFModelID)),
	)

	fetchStarted := time.Now()
	logger.Info(
		"huggingface source fetch started",
		slog.String("requestedRevision", strings.TrimSpace(options.Revision)),
		slog.Bool("sourceMirrorEnabled", remoteSourceMirror(options) != nil),
	)

	remote, cleanupDir, err := fetchRemote(ctx, options, "ai-model-hf-publish-")
	if err != nil {
		return publicationartifact.Result{}, err
	}
	defer cleanupDir()

	logger.Info(
		"huggingface source fetch completed",
		slog.Int64("durationMs", time.Since(fetchStarted).Milliseconds()),
		slog.String("resolvedRevision", strings.TrimSpace(remote.Provenance.ResolvedRevision)),
		slog.String("resolvedInputFormat", strings.TrimSpace(string(remote.InputFormat))),
		slog.Int("selectedFileCount", len(remote.SelectedFiles)),
		slog.Int64("sourceMirrorObjectCount", sourceMirrorObjectCount(remote.SourceMirror)),
		slog.Int64("sourceMirrorSizeBytes", sourceMirrorSizeBytes(remote.SourceMirror)),
	)
	if len(remote.SelectedFiles) > 0 {
		logger.Debug("huggingface selected file sample", slog.Any("selectedFilesSample", sampleStrings(remote.SelectedFiles, 8)))
	}

	resolvedProfile, publishResult, err := resolveAndPublish(ctx, options, remote.ModelDir, remote.InputFormat, sourceProfileInput{
		Task:     options.Task,
		TaskHint: remote.Fallbacks.TaskHint,
		Provenance: sourceProfileProvenance{
			License:      remote.Metadata.License,
			SourceRepoID: remote.Metadata.SourceRepoID,
		},
	})
	if err != nil {
		return publicationartifact.Result{}, err
	}

	rawSource := sourceMirrorRawProvenance(options, remote.SourceMirror)

	result := buildBackendResult(
		publicationdata.SourceProvenance{
			Type:              modelsv1alpha1.ModelSourceTypeHuggingFace,
			ExternalReference: remote.Provenance.ExternalReference,
			ResolvedRevision:  remote.Provenance.ResolvedRevision,
			RawURI:            rawSource.RawURI,
			RawObjectCount:    rawSource.RawObjectCount,
			RawSizeBytes:      rawSource.RawSizeBytes,
		},
		resolvedProfile,
		publishResult,
	)
	return attachBackendSourceMirror(result, remote.SourceMirror), nil
}

func fetchRemote(ctx context.Context, options Options, prefix string) (sourcefetch.RemoteResult, func(), error) {
	workspace, cleanupDir, err := ensureWorkspace(options.SnapshotDir, prefix)
	if err != nil {
		return sourcefetch.RemoteResult{}, nil, err
	}

	slog.Default().Debug("huggingface publication workspace prepared", slog.String("workspace", workspace))

	remote, err := sourcefetch.FetchRemoteModel(ctx, sourcefetch.RemoteOptions{
		URL:             huggingFaceSourceURL(options.HFModelID, options.Revision),
		Workspace:       workspace,
		RequestedFormat: options.InputFormat,
		HFToken:         options.HFToken,
		SourceMirror:    remoteSourceMirror(options),
	})
	if err != nil {
		cleanupDir()
		return sourcefetch.RemoteResult{}, nil, err
	}

	return remote, cleanupDir, nil
}

func huggingFaceSourceURL(repoID, revision string) string {
	base := "https://huggingface.co/" + strings.Trim(strings.TrimSpace(repoID), "/")
	if strings.TrimSpace(revision) == "" {
		return base
	}
	return base + "?revision=" + strings.TrimSpace(revision)
}

func attachBackendSourceMirror(
	result publicationartifact.Result,
	sourceMirror *sourcefetch.SourceMirrorSnapshot,
) publicationartifact.Result {
	if sourceMirror == nil || result.CleanupHandle.Backend == nil {
		return result
	}
	result.CleanupHandle.Backend.SourceMirrorPrefix = strings.TrimSpace(sourceMirror.CleanupPrefix)
	return result
}

func sourceMirrorObjectCount(snapshot *sourcefetch.SourceMirrorSnapshot) int64 {
	if snapshot == nil {
		return 0
	}
	return snapshot.ObjectCount
}

func sourceMirrorSizeBytes(snapshot *sourcefetch.SourceMirrorSnapshot) int64 {
	if snapshot == nil {
		return 0
	}
	return snapshot.SizeBytes
}

func sampleStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return append([]string(nil), values...)
	}
	sample := append([]string(nil), values[:limit]...)
	return append(sample, "...")
}
