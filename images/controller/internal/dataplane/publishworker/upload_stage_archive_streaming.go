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
	"io"
	"log/slog"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
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
	if !isArchiveUploadPath(fileName) {
		return publicationartifact.Result{}, false, nil
	}

	inspection, archiveSize, err := inspectUploadStageArchive(ctx, options, options.UploadStaging, fileName)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if !supportsArchiveUpload(inspection) {
		return publicationartifact.Result{}, false, nil
	}

	result, err := publishUploadArchive(ctx, options, logger, uploadArchivePublication{
		sourcePath:      strings.TrimSpace(options.UploadStage.Key),
		artifactURI:     rawURI(options.UploadStage.Bucket, options.UploadStage.Key),
		compressionPath: fileName,
		inspection:      inspection,
		reader: uploadStagingObjectReader{
			bucket: strings.TrimSpace(options.UploadStage.Bucket),
			reader: options.UploadStaging,
		},
		sizeBytes:  archiveSize,
		logMessage: "upload stage archive object-source path selected",
		logArgs:    []any{slog.String("uploadStageFileName", fileName)},
	})
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	return result, true, nil
}

func inspectUploadStageArchive(
	ctx context.Context,
	options Options,
	reader uploadstagingports.Reader,
	fileName string,
) (sourcefetch.ArchiveInspection, int64, error) {
	bucket := strings.TrimSpace(options.UploadStage.Bucket)
	key := strings.TrimSpace(options.UploadStage.Key)

	if archiveio.IsZipArchive(fileName) {
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
			archiveio.NewRangeReaderAt(ctx, key, stat.SizeBytes, func(ctx context.Context, sourcePath string, offset, length int64) (io.ReadCloser, error) {
				output, err := rangeReader.OpenReadRange(ctx, uploadstagingports.OpenReadRangeInput{
					Bucket: bucket,
					Key:    sourcePath,
					Offset: offset,
					Length: length,
				})
				if err != nil {
					return nil, err
				}
				return output.Body, nil
			}),
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
