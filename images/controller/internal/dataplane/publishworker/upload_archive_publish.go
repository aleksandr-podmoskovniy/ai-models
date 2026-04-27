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

type uploadArchivePublication struct {
	sourcePath      string
	artifactURI     string
	compressionPath string
	inspection      sourcefetch.ArchiveInspection
	reader          modelpackports.PublishObjectReader
	sizeBytes       int64
	logMessage      string
	logArgs         []any
}

func publishUploadArchive(
	ctx context.Context,
	options Options,
	logger *slog.Logger,
	input uploadArchivePublication,
) (publicationartifact.Result, error) {
	layerCompression := uploadArchiveLayerCompression(input.compressionPath)
	if logger != nil {
		logger.Info(input.logMessage, append(input.logArgs,
			slog.String("resolvedInputFormat", string(input.inspection.InputFormat)),
			slog.Int("selectedFileCount", len(input.inspection.SelectedFiles)),
			slog.String("archiveRootPrefix", strings.TrimSpace(input.inspection.RootPrefix)),
			slog.String("archiveLayerCompression", string(layerCompression)),
		)...)
	}

	preResolved, err := resolveArchiveInspectionSummary(options, input.inspection)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	resolvedProfile, publishResult, err := resolveAndPublishWithLayers(
		ctx,
		options,
		input.artifactURI,
		input.inspection.InputFormat,
		sourceProfileInput{Task: options.Task},
		[]modelpackports.PublishLayer{
			{
				SourcePath:  input.sourcePath,
				TargetPath:  modelpackports.MaterializedModelPathName,
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatTar,
				Compression: layerCompression,
				Archive: &modelpackports.PublishArchiveSource{
					StripPathPrefix: input.inspection.RootPrefix,
					SelectedFiles:   append([]string(nil), input.inspection.SelectedFiles...),
					Reader:          input.reader,
					SizeBytes:       input.sizeBytes,
				},
			},
		},
		&preResolved,
	)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	if err := cleanupStagedUploadObject(ctx, options, logger); err != nil {
		return publicationartifact.Result{}, err
	}
	return buildUploadResult(options, resolvedProfile, publishResult), nil
}

func supportsArchiveUpload(inspection sourcefetch.ArchiveInspection) bool {
	return inspection.InputFormat == modelsv1alpha1.ModelInputFormatSafetensors ||
		inspection.InputFormat == modelsv1alpha1.ModelInputFormatGGUF
}

func resolveArchiveInspectionSummary(
	options Options,
	inspection sourcefetch.ArchiveInspection,
) (publicationdata.ResolvedProfile, error) {
	switch inspection.InputFormat {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return safetensorsprofile.ResolveSummary(safetensorsprofile.SummaryInput{
			ConfigPayload:          inspection.ConfigPayload,
			WeightBytes:            inspection.WeightStats.TotalBytes,
			LargestWeightFileBytes: inspection.WeightStats.LargestFileBytes,
			WeightFileCount:        inspection.WeightStats.FileCount,
			Task:                   options.Task,
		})
	case modelsv1alpha1.ModelInputFormatGGUF:
		return ggufprofile.ResolveSummary(ggufprofile.SummaryInput{
			ModelFileName:  inspection.ModelFile,
			ModelSizeBytes: inspection.ModelFileSize,
			Task:           options.Task,
		})
	default:
		return publicationdata.ResolvedProfile{}, errors.New("unsupported archive inspection format")
	}
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
