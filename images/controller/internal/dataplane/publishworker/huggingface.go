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
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func publishFromHuggingFace(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.HFModelID) == "" {
		return publicationartifact.Result{}, errors.New("hf-model-id is required")
	}
	remote, cleanupDir, err := fetchRemote(ctx, options, "ai-model-hf-publish-")
	if err != nil {
		return publicationartifact.Result{}, err
	}
	defer cleanupDir()

	resolvedProfile, publishResult, err := resolveAndPublish(ctx, options, remote.ModelDir, remote.InputFormat, sourceProfileInput{
		Task:           options.Task,
		TaskHint:       remote.Fallbacks.TaskHint,
		RuntimeEngines: options.RuntimeEngines,
		Provenance: sourceProfileProvenance{
			License:      remote.Metadata.License,
			SourceRepoID: remote.Metadata.SourceRepoID,
		},
	}, fmt.Sprintf("Published from Hugging Face source %s", options.HFModelID))
	if err != nil {
		cleanupErr := cleanupRemoteStagedObjects(ctx, options, remote.StagedObjects)
		if cleanupErr != nil {
			return publicationartifact.Result{}, errors.Join(err, cleanupErr)
		}
		return publicationartifact.Result{}, err
	}
	if err := cleanupRemoteStagedObjects(ctx, options, remote.StagedObjects); err != nil {
		return publicationartifact.Result{}, err
	}
	rawSource := remoteRawProvenance(options, remote.StagedObjects)
	if remote.SourceMirror != nil {
		rawSource = sourceMirrorRawProvenance(options, remote.SourceMirror)
	}

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

	remote, err := sourcefetch.FetchRemoteModel(ctx, sourcefetch.RemoteOptions{
		URL:             huggingFaceSourceURL(options.HFModelID, options.Revision),
		Workspace:       workspace,
		RequestedFormat: options.InputFormat,
		HFToken:         options.HFToken,
		RawStage:        remoteRawStage(options),
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
