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
	"net/http"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type RawStageOptions struct {
	Bucket    string
	KeyPrefix string
	Client    uploadstagingports.Client
}

type RemoteOptions struct {
	URL             string
	Workspace       string
	RequestedFormat modelsv1alpha1.ModelInputFormat
	HFToken         string
	RawStage        *RawStageOptions
	SourceMirror    *SourceMirrorOptions
}

type SourceMirrorOptions struct {
	Bucket           string
	Client           uploadstagingports.Client
	UploadHTTPClient *http.Client
	Store            sourcemirrorports.Store
	BasePrefix       string
}

type RemoteResult struct {
	SourceType    modelsv1alpha1.ModelSourceType
	ModelDir      string
	InputFormat   modelsv1alpha1.ModelInputFormat
	Provenance    RemoteProvenance
	Fallbacks     RemoteProfileFallbacks
	Metadata      RemoteMetadata
	StagedObjects []cleanuphandle.UploadStagingHandle
	SourceMirror  *SourceMirrorSnapshot
}

type SourceMirrorSnapshot struct {
	Locator       sourcemirrorports.SnapshotLocator
	CleanupPrefix string
	SizeBytes     int64
	ObjectCount   int64
}

type RemoteProvenance struct {
	ExternalReference string
	ResolvedRevision  string
}

type RemoteProfileFallbacks struct {
	TaskHint string
}

type RemoteMetadata struct {
	License      string
	SourceRepoID string
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
	default:
		return RemoteResult{}, errors.New("unsupported remote source type")
	}
}

func resolveRemoteFormat(files []string, requested modelsv1alpha1.ModelInputFormat) (modelsv1alpha1.ModelInputFormat, error) {
	if strings.TrimSpace(string(requested)) != "" {
		return requested, nil
	}
	return modelformat.DetectRemoteFormat(files)
}
