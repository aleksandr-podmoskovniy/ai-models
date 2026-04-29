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
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	publicationartifact "github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func publishFromOllama(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.SourceURL) == "" {
		return publicationartifact.Result{}, errors.New("source-url is required for Ollama source")
	}

	logger := slog.Default().With(
		slog.String("sourceType", string(modelsv1alpha1.ModelSourceTypeOllama)),
		slog.String("sourceURL", strings.TrimSpace(options.SourceURL)),
	)
	fetchStarted := time.Now()
	logger.Info("ollama source fetch started", slog.Bool("sourceMirrorEnabled", remoteSourceMirror(options) != nil))
	remote, err := fetchRemoteModelFunc(ctx, sourcefetch.RemoteOptions{
		URL:                      strings.TrimSpace(options.SourceURL),
		RequestedFormat:          options.InputFormat,
		SourceMirror:             remoteSourceMirror(options),
		StorageReservation:       options.StorageReservation,
		SkipLocalMaterialization: true,
	})
	if err != nil {
		return publicationartifact.Result{}, err
	}
	logger.Info(
		"ollama source fetch completed",
		slog.Int64("durationMs", time.Since(fetchStarted).Milliseconds()),
		slog.String("resolvedRevision", strings.TrimSpace(remote.Provenance.ResolvedRevision)),
		slog.String("resolvedInputFormat", strings.TrimSpace(string(remote.InputFormat))),
		slog.Int("selectedFileCount", len(remote.SelectedFiles)),
		slog.Int64("sourceMirrorObjectCount", sourceMirrorObjectCount(remote.SourceMirror)),
		slog.Int64("sourceMirrorSizeBytes", sourceMirrorSizeBytes(remote.SourceMirror)),
	)

	preResolved, err := resolveRemoteProfile(options, remote)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	publishLayers, err := buildRemoteObjectPublishLayers(ctx, options, remote, ollamaArtifactURI(remote))
	if err != nil {
		return publicationartifact.Result{}, err
	}

	modelInputPath := remote.ModelDir
	if len(publishLayers) > 0 {
		switch {
		case remote.SourceMirror != nil:
			modelInputPath = sourceMirrorArtifactURI(options, remote.SourceMirror)
		case remote.ObjectSource != nil:
			modelInputPath = ollamaArtifactURI(remote)
		}
	}

	resolvedProfile, publishResult, err := resolveAndPublishWithLayers(ctx, options, modelInputPath, remote.InputFormat, sourceProfileInput{
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
			Type:              modelsv1alpha1.ModelSourceTypeOllama,
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

func buildRemoteObjectPublishLayers(
	ctx context.Context,
	options Options,
	remote sourcefetch.RemoteResult,
	directArtifactURI string,
) ([]modelpackports.PublishLayer, error) {
	if remote.ObjectSource != nil {
		return buildObjectSourcePublishLayers(
			directArtifactURI,
			remoteObjectReader{reader: remote.ObjectSource.Reader},
			mapRemoteObjectFiles(remote.ObjectSource.Files),
		)
	}
	if strings.TrimSpace(remote.ModelDir) != "" || remote.SourceMirror == nil {
		return nil, nil
	}
	files, err := sourceMirrorPublishFiles(ctx, options, remote.SourceMirror, remote.SelectedFiles)
	if err != nil {
		return nil, err
	}
	return buildObjectSourcePublishLayers(
		sourceMirrorArtifactURI(options, remote.SourceMirror),
		uploadStagingObjectReader{
			bucket: strings.TrimSpace(options.RawStageBucket),
			reader: options.UploadStaging,
		},
		files,
	)
}

func ollamaArtifactURI(remote sourcefetch.RemoteResult) string {
	reference := strings.TrimSpace(remote.Provenance.ExternalReference)
	if reference == "" {
		return ""
	}
	if strings.HasPrefix(reference, "https://") {
		return reference
	}
	return "https://" + reference
}
