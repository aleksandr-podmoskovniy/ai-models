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
	"path/filepath"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

var (
	huggingFaceBaseURL       = "https://huggingface.co"
	fetchHuggingFaceInfoFunc = FetchHuggingFaceInfo
)

type HuggingFaceInfo struct {
	ID          string
	SHA         string
	PipelineTag string
	License     string
	Files       []string
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
	License string `json:"license"`
}

func fetchHuggingFaceModel(ctx context.Context, options RemoteOptions) (RemoteResult, error) {
	repoID, revision, err := modelsv1alpha1.ParseHuggingFaceURL(options.URL)
	if err != nil {
		return RemoteResult{}, err
	}

	info, err := fetchHuggingFaceInfoFunc(ctx, repoID, revision, options.HFToken)
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

	resolvedRevision := ResolveHuggingFaceRevision(info, revision)
	sourceMirrorSnapshot, err := persistHuggingFaceMirrorManifest(ctx, options.SourceMirror, repoID, resolvedRevision, selectedFiles)
	if err != nil {
		return RemoteResult{}, err
	}
	snapshotDir := filepath.Join(options.Workspace, ".hf-snapshot")
	if sourceMirrorSnapshot != nil {
		if err := mirrorHuggingFaceSnapshotFiles(ctx, options.SourceMirror, repoID, resolvedRevision, options.HFToken, selectedFiles, sourceMirrorSnapshot); err != nil {
			_ = persistHuggingFaceMirrorPhase(ctx, options.SourceMirror, sourceMirrorSnapshot, sourcemirrorports.SnapshotPhaseFailed, nil, err.Error())
			return RemoteResult{}, err
		}
		if err := materializeHuggingFaceMirrorSnapshot(ctx, options.SourceMirror, sourceMirrorSnapshot, snapshotDir, selectedFiles); err != nil {
			return RemoteResult{}, err
		}
	} else {
		if err := newHuggingFaceSnapshotDownloader().Download(ctx, huggingFaceSnapshotDownloadInput{
			RepoID:      repoID,
			Revision:    resolvedRevision,
			Token:       options.HFToken,
			Files:       selectedFiles,
			SnapshotDir: snapshotDir,
		}); err != nil {
			return RemoteResult{}, err
		}
	}

	modelDir := filepath.Join(options.Workspace, "checkpoint")
	var stagedObjects []cleanuphandle.UploadStagingHandle
	if sourceMirrorSnapshot == nil && rawStageEnabled(options.RawStage) {
		stagedObjects, err = stageHuggingFaceSnapshotFiles(ctx, snapshotDir, selectedFiles, *options.RawStage)
		if err != nil {
			return RemoteResult{}, err
		}
	}
	if err := materializeHuggingFaceSnapshot(snapshotDir, modelDir, selectedFiles); err != nil {
		return RemoteResult{}, err
	}
	if err := modelformat.ValidateDir(modelDir, inputFormat); err != nil {
		return RemoteResult{}, err
	}

	return RemoteResult{
		SourceType:  modelsv1alpha1.ModelSourceTypeHuggingFace,
		ModelDir:    modelDir,
		InputFormat: inputFormat,
		Provenance: RemoteProvenance{
			ExternalReference: firstNonEmpty(info.ID, repoID),
			ResolvedRevision:  resolvedRevision,
		},
		Fallbacks: RemoteProfileFallbacks{
			TaskHint: info.PipelineTag,
		},
		Metadata: RemoteMetadata{
			License:      info.License,
			SourceRepoID: info.ID,
		},
		StagedObjects: stagedObjects,
		SourceMirror:  sourceMirrorSnapshot,
	}, nil
}
