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
	"fmt"
	"path"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func buildBackendResult(
	source publicationdata.SourceProvenance,
	resolved publicationdata.ResolvedProfile,
	publishResult modelpackports.PublishResult,
) publicationartifact.Result {
	repositoryMetadataPrefix := backendRepositoryMetadataPrefix(publishResult.Reference)
	return publicationartifact.Result{
		Source: source,
		Artifact: publicationdata.PublishedArtifact{
			Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:       publishResult.Reference,
			Digest:    publishResult.Digest,
			MediaType: publishResult.MediaType,
			SizeBytes: publishResult.SizeBytes,
		},
		Resolved: resolved,
		CleanupHandle: cleanuphandle.Handle{
			Kind: cleanuphandle.KindBackendArtifact,
			Artifact: &cleanuphandle.ArtifactSnapshot{
				Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:    publishResult.Reference,
				Digest: publishResult.Digest,
			},
			Backend: &cleanuphandle.BackendArtifactHandle{
				Reference:                publishResult.Reference,
				RepositoryMetadataPrefix: repositoryMetadataPrefix,
			},
		},
	}
}

func backendRepositoryMetadataPrefix(reference string) string {
	repository := repositoryPathFromOCIReference(reference)
	if repository == "" {
		return ""
	}
	return path.Join("dmcr", "docker", "registry", "v2", "repositories", repository)
}

func repositoryPathFromOCIReference(reference string) string {
	cleanReference := strings.TrimSpace(strings.SplitN(reference, "@", 2)[0])
	registry, repository, found := strings.Cut(cleanReference, "/")
	if !found || strings.TrimSpace(registry) == "" {
		return ""
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return ""
	}
	repositoryPart := repository[strings.LastIndex(repository, "/")+1:]
	if strings.Contains(repositoryPart, ":") {
		repository = repository[:strings.LastIndex(repository, ":")]
	}
	return strings.Trim(repository, "/")
}

func run(ctx context.Context, options Options) (publicationartifact.Result, error) {
	switch options.SourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace:
		return publishFromHuggingFace(ctx, options)
	case modelsv1alpha1.ModelSourceTypeUpload:
		return publishFromUpload(ctx, options)
	default:
		return publicationartifact.Result{}, fmt.Errorf("unsupported publish worker source type %q", options.SourceType)
	}
}

func resolveUploadInputFormat(checkpointDir string, requested modelsv1alpha1.ModelInputFormat) (modelsv1alpha1.ModelInputFormat, error) {
	if strings.TrimSpace(string(requested)) != "" {
		return requested, nil
	}
	return modelformat.DetectPathFormat(checkpointDir)
}
