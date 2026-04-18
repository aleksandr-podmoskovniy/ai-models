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
	"log/slog"
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	ggufprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/gguf"
	safetensorsprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/safetensors"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func tryPublishStreamingUploadArchive(
	ctx context.Context,
	options Options,
	uploadPath string,
	logger *slog.Logger,
) (publicationartifact.Result, bool, error) {
	if !isStreamingArchiveUploadPath(uploadPath) {
		return publicationartifact.Result{}, false, nil
	}

	inspection, err := sourcefetch.InspectModelArchive(uploadPath, options.InputFormat)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if inspection.InputFormat != modelsv1alpha1.ModelInputFormatSafetensors &&
		inspection.InputFormat != modelsv1alpha1.ModelInputFormatGGUF {
		return publicationartifact.Result{}, false, nil
	}

	layerCompression := uploadArchiveLayerCompression(uploadPath)
	logger.Info(
		"upload archive streaming path selected",
		slog.String("uploadPath", uploadPath),
		slog.String("resolvedInputFormat", string(inspection.InputFormat)),
		slog.Int("selectedFileCount", len(inspection.SelectedFiles)),
		slog.String("archiveRootPrefix", strings.TrimSpace(inspection.RootPrefix)),
		slog.String("archiveLayerCompression", string(layerCompression)),
	)

	var resolvedProfile publicationdata.ResolvedProfile
	switch inspection.InputFormat {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		resolvedProfile, err = safetensorsprofile.ResolveSummary(safetensorsprofile.SummaryInput{
			ConfigPayload: inspection.ConfigPayload,
			WeightBytes:   inspection.WeightBytes,
			Task:          options.Task,
		})
	case modelsv1alpha1.ModelInputFormatGGUF:
		resolvedProfile, err = ggufprofile.ResolveSummary(ggufprofile.SummaryInput{
			ModelFileName:  inspection.ModelFile,
			ModelSizeBytes: inspection.ModelFileSize,
			Task:           options.Task,
		})
	default:
		return publicationartifact.Result{}, false, nil
	}
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	publishLayers := []modelpackports.PublishLayer{
		{
			SourcePath:  uploadPath,
			TargetPath:  modelpackports.MaterializedModelPathName,
			Base:        modelpackports.LayerBaseModel,
			Format:      modelpackports.LayerFormatTar,
			Compression: layerCompression,
			Archive: &modelpackports.PublishArchiveSource{
				StripPathPrefix: inspection.RootPrefix,
				SelectedFiles:   append([]string(nil), inspection.SelectedFiles...),
			},
		},
	}
	resolvedProfile, publishResult, err := resolveAndPublishWithLayers(
		ctx,
		options,
		uploadPath,
		inspection.InputFormat,
		sourceProfileInput{Task: options.Task},
		publishLayers,
		&resolvedProfile,
	)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if err := cleanupStagedUploadObject(ctx, options, logger); err != nil {
		return publicationartifact.Result{}, false, err
	}

	return buildUploadResult(options, resolvedProfile, publishResult), true, nil
}

func isStreamingArchiveUploadPath(uploadPath string) bool {
	lowerPath := strings.ToLower(strings.TrimSpace(uploadPath))
	return strings.HasSuffix(lowerPath, ".tar") ||
		strings.HasSuffix(lowerPath, ".tar.gz") ||
		strings.HasSuffix(lowerPath, ".tgz") ||
		strings.HasSuffix(lowerPath, ".tar.zst") ||
		strings.HasSuffix(lowerPath, ".tar.zstd") ||
		strings.HasSuffix(lowerPath, ".tzst") ||
		strings.HasSuffix(lowerPath, ".zip")
}

func uploadArchiveLayerCompression(uploadPath string) modelpackports.LayerCompression {
	lowerPath := strings.ToLower(strings.TrimSpace(filepath.Base(uploadPath)))
	if strings.HasSuffix(lowerPath, ".tar.gz") || strings.HasSuffix(lowerPath, ".tgz") {
		return modelpackports.LayerCompressionGzip
	}
	if strings.HasSuffix(lowerPath, ".tar.zst") || strings.HasSuffix(lowerPath, ".tar.zstd") || strings.HasSuffix(lowerPath, ".tzst") {
		return modelpackports.LayerCompressionZstd
	}
	return modelpackports.LayerCompressionNone
}
