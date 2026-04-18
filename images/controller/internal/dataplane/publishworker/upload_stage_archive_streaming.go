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
	"io"
	"log/slog"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	ggufprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/gguf"
	safetensorsprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/safetensors"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func tryPublishUploadStageStreamingArchive(
	ctx context.Context,
	options Options,
	logger *slog.Logger,
) (publicationartifact.Result, bool, error) {
	if options.UploadStage == nil || strings.TrimSpace(options.UploadPath) != "" {
		return publicationartifact.Result{}, false, nil
	}

	fileName, err := uploadStageFileName(options)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if !isRemoteArchiveUploadPath(fileName) {
		return publicationartifact.Result{}, false, nil
	}

	inspection, archiveSize, err := inspectUploadStageArchive(ctx, options, options.UploadStaging, fileName)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if inspection.InputFormat != modelsv1alpha1.ModelInputFormatSafetensors &&
		inspection.InputFormat != modelsv1alpha1.ModelInputFormatGGUF {
		return publicationartifact.Result{}, false, nil
	}

	layerCompression := uploadArchiveLayerCompression(fileName)
	logger.Info(
		"upload stage archive object-source path selected",
		slog.String("resolvedInputFormat", string(inspection.InputFormat)),
		slog.String("uploadStageFileName", fileName),
		slog.Int("selectedFileCount", len(inspection.SelectedFiles)),
		slog.String("archiveRootPrefix", strings.TrimSpace(inspection.RootPrefix)),
		slog.String("archiveLayerCompression", string(layerCompression)),
	)

	preResolved, err := resolveArchiveInspectionSummary(options, inspection)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	publishLayers := []modelpackports.PublishLayer{
		{
			SourcePath:  strings.TrimSpace(options.UploadStage.Key),
			TargetPath:  modelpackports.MaterializedModelPathName,
			Base:        modelpackports.LayerBaseModel,
			Format:      modelpackports.LayerFormatTar,
			Compression: layerCompression,
			Archive: &modelpackports.PublishArchiveSource{
				StripPathPrefix: inspection.RootPrefix,
				SelectedFiles:   append([]string(nil), inspection.SelectedFiles...),
				Reader: uploadStagingObjectReader{
					bucket: strings.TrimSpace(options.UploadStage.Bucket),
					reader: options.UploadStaging,
				},
				SizeBytes: archiveSize,
			},
		},
	}
	resolvedProfile, publishResult, err := resolveAndPublishWithLayers(
		ctx,
		options,
		rawURI(options.UploadStage.Bucket, options.UploadStage.Key),
		inspection.InputFormat,
		sourceProfileInput{Task: options.Task},
		publishLayers,
		&preResolved,
	)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if err := cleanupStagedUploadObject(ctx, options, logger); err != nil {
		return publicationartifact.Result{}, false, err
	}
	return buildUploadResult(options, resolvedProfile, publishResult), true, nil
}

func inspectUploadStageArchive(
	ctx context.Context,
	options Options,
	reader uploadstagingports.Reader,
	fileName string,
) (sourcefetch.ArchiveInspection, int64, error) {
	bucket := strings.TrimSpace(options.UploadStage.Bucket)
	key := strings.TrimSpace(options.UploadStage.Key)

	if isZipArchiveUploadPath(fileName) {
		rangeReader, ok := options.UploadStaging.(uploadstagingports.RangeReader)
		if !ok {
			return sourcefetch.ArchiveInspection{}, 0, nil
		}
		stat, err := options.UploadStaging.Stat(ctx, uploadstagingports.StatInput{Bucket: bucket, Key: key})
		if err != nil {
			return sourcefetch.ArchiveInspection{}, 0, err
		}
		inspection, err := sourcefetch.InspectZipModelArchiveReaderAt(
			fileName,
			stat.SizeBytes,
			uploadStageArchiveReaderAt{
				ctx:         ctx,
				bucket:      bucket,
				key:         key,
				sizeBytes:   stat.SizeBytes,
				rangeReader: rangeReader,
			},
			options.InputFormat,
		)
		return inspection, stat.SizeBytes, err
	}

	inspection, err := sourcefetch.InspectTarModelArchiveReader(fileName, func() (io.ReadCloser, error) {
		output, err := reader.OpenRead(ctx, uploadstagingports.OpenReadInput{Bucket: bucket, Key: key})
		if err != nil {
			return nil, err
		}
		return output.Body, nil
	}, options.InputFormat)
	return inspection, 0, err
}

func resolveArchiveInspectionSummary(
	options Options,
	inspection sourcefetch.ArchiveInspection,
) (publicationdata.ResolvedProfile, error) {
	switch inspection.InputFormat {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return safetensorsprofile.ResolveSummary(safetensorsprofile.SummaryInput{
			ConfigPayload: inspection.ConfigPayload,
			WeightBytes:   inspection.WeightBytes,
			Task:          options.Task,
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

func isRemoteArchiveUploadPath(uploadPath string) bool {
	lowerPath := strings.ToLower(strings.TrimSpace(uploadPath))
	return strings.HasSuffix(lowerPath, ".tar") ||
		strings.HasSuffix(lowerPath, ".tar.gz") ||
		strings.HasSuffix(lowerPath, ".tgz") ||
		strings.HasSuffix(lowerPath, ".tar.zst") ||
		strings.HasSuffix(lowerPath, ".tar.zstd") ||
		strings.HasSuffix(lowerPath, ".tzst") ||
		strings.HasSuffix(lowerPath, ".zip")
}

func isZipArchiveUploadPath(uploadPath string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(uploadPath)), ".zip")
}

type uploadStageArchiveReaderAt struct {
	ctx         context.Context
	bucket      string
	key         string
	sizeBytes   int64
	rangeReader uploadstagingports.RangeReader
}

func (r uploadStageArchiveReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if offset < 0 || offset >= r.sizeBytes {
		return 0, io.EOF
	}

	length := int64(len(p))
	if remaining := r.sizeBytes - offset; length > remaining {
		length = remaining
	}
	output, err := r.rangeReader.OpenReadRange(r.ctx, uploadstagingports.OpenReadRangeInput{
		Bucket: r.bucket,
		Key:    r.key,
		Offset: offset,
		Length: length,
	})
	if err != nil {
		return 0, err
	}
	defer output.Body.Close()

	n, err := io.ReadFull(output.Body, p[:length])
	switch err {
	case nil:
		if int64(n) < int64(len(p)) {
			return n, io.EOF
		}
		return n, nil
	case io.ErrUnexpectedEOF, io.EOF:
		if n == 0 {
			return 0, io.EOF
		}
		return n, io.EOF
	default:
		return n, err
	}
}
