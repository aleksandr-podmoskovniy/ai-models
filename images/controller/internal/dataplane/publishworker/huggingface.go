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
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

var fetchRemoteModelFunc = sourcefetch.FetchRemoteModel

func publishFromHuggingFace(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.HFModelID) == "" {
		return publicationartifact.Result{}, errors.New("hf-model-id is required")
	}

	logger := slog.Default().With(
		slog.String("sourceType", string(modelsv1alpha1.ModelSourceTypeHuggingFace)),
		slog.String("sourceRepoID", strings.TrimSpace(options.HFModelID)),
		slog.String("sourceFetchMode", string(publicationports.NormalizeSourceFetchMode(options.SourceFetchMode))),
	)

	fetchStarted := time.Now()
	logger.Info(
		"huggingface source fetch started",
		slog.String("requestedRevision", strings.TrimSpace(options.Revision)),
		slog.Bool("sourceMirrorEnabled", remoteSourceMirror(options) != nil),
	)

	remote, err := fetchRemote(ctx, options)
	if err != nil {
		return publicationartifact.Result{}, err
	}

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

	preResolved, err := resolveRemoteProfile(options, remote)
	if err != nil {
		return publicationartifact.Result{}, err
	}

	modelInputPath := remote.ModelDir
	publishLayers, err := buildHuggingFacePublishLayers(ctx, options, remote)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	if len(publishLayers) > 0 {
		switch {
		case remote.SourceMirror != nil:
			modelInputPath = sourceMirrorArtifactURI(options, remote.SourceMirror)
		case remote.ObjectSource != nil:
			modelInputPath = huggingFaceArtifactURI(remote)
		}
	}

	resolvedProfile, publishResult, err := resolveAndPublishWithLayers(ctx, options, modelInputPath, remote.InputFormat, sourceProfileInput{
		Task:               options.Task,
		SourceDeclaredTask: remote.Fallbacks.SourceDeclaredTask,
		TaskHint:           remote.Fallbacks.TaskHint,
		Provenance: sourceProfileProvenance{
			License:      remote.Metadata.License,
			SourceRepoID: remote.Metadata.SourceRepoID,
		},
	}, publishLayers, preResolved)
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

func fetchRemote(ctx context.Context, options Options) (sourcefetch.RemoteResult, error) {
	return fetchRemoteAttempt(ctx, options)
}

func fetchRemoteAttempt(ctx context.Context, options Options) (sourcefetch.RemoteResult, error) {
	return fetchRemoteModelFunc(ctx, sourcefetch.RemoteOptions{
		URL:                      huggingFaceSourceURL(options.HFModelID, options.Revision),
		RequestedFormat:          options.InputFormat,
		HFToken:                  options.HFToken,
		SourceMirror:             remoteSourceMirror(options),
		StorageReservation:       options.StorageReservation,
		SkipLocalMaterialization: true,
	})
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
