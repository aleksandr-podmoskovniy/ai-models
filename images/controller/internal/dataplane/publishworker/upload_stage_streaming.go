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
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	ggufprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/gguf"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
)

func tryPublishUploadStageDirectObjectSource(
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
	if isArchiveUploadPath(fileName) {
		return publicationartifact.Result{}, false, nil
	}

	inputFormat, err := resolveDirectUploadStageInputFormat(ctx, options, options.UploadStaging, fileName)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if inputFormat != modelsv1alpha1.ModelInputFormatGGUF {
		return publicationartifact.Result{}, false, nil
	}

	stat, err := options.UploadStaging.Stat(ctx, uploadstagingports.StatInput{
		Bucket: strings.TrimSpace(options.UploadStage.Bucket),
		Key:    strings.TrimSpace(options.UploadStage.Key),
	})
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if stat.SizeBytes <= 0 {
		return publicationartifact.Result{}, false, errors.New("upload staging object size must be positive")
	}
	if err := validateUploadStageGGUF(ctx, options, options.UploadStaging); err != nil {
		return publicationartifact.Result{}, false, err
	}

	logger.Info(
		"upload stage direct object-source path selected",
		slog.String("resolvedInputFormat", string(inputFormat)),
		slog.String("uploadStageFileName", fileName),
		slog.Int64("uploadSizeBytes", stat.SizeBytes),
	)

	preResolved, err := ggufprofile.ResolveSummary(ggufprofile.SummaryInput{
		ModelFileName:  fileName,
		ModelSizeBytes: stat.SizeBytes,
		Task:           options.Task,
	})
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	publishLayers := []modelpackports.PublishLayer{
		{
			SourcePath:  rawURI(options.UploadStage.Bucket, options.UploadStage.Key),
			TargetPath:  modelpackports.MaterializedModelPathName,
			Base:        modelpackports.LayerBaseModel,
			Format:      modelpackports.LayerFormatTar,
			Compression: modelpackports.LayerCompressionNone,
			ObjectSource: &modelpackports.PublishObjectSource{
				Reader: uploadStagingObjectReader{
					bucket: strings.TrimSpace(options.UploadStage.Bucket),
					reader: options.UploadStaging,
				},
				Files: []modelpackports.PublishObjectFile{
					{
						SourcePath: strings.TrimSpace(options.UploadStage.Key),
						TargetPath: fileName,
						SizeBytes:  stat.SizeBytes,
						ETag:       strings.TrimSpace(stat.ETag),
					},
				},
			},
		},
	}

	resolvedProfile, publishResult, err := resolveAndPublishWithLayers(
		ctx,
		options,
		rawURI(options.UploadStage.Bucket, options.UploadStage.Key),
		inputFormat,
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

func resolveDirectUploadStageInputFormat(
	ctx context.Context,
	options Options,
	reader uploadstagingports.Reader,
	fileName string,
) (modelsv1alpha1.ModelInputFormat, error) {
	if strings.TrimSpace(string(options.InputFormat)) != "" {
		return options.InputFormat, nil
	}
	if strings.HasSuffix(strings.ToLower(fileName), ".gguf") {
		return modelsv1alpha1.ModelInputFormatGGUF, nil
	}
	looksLikeGGUF, err := uploadStageHasGGUFMagic(ctx, options, reader)
	if err != nil {
		return "", err
	}
	if looksLikeGGUF {
		return modelsv1alpha1.ModelInputFormatGGUF, nil
	}
	return "", nil
}

func validateUploadStageGGUF(
	ctx context.Context,
	options Options,
	reader uploadstagingports.Reader,
) error {
	looksLikeGGUF, err := uploadStageHasGGUFMagic(ctx, options, reader)
	if err != nil {
		return err
	}
	if !looksLikeGGUF {
		return errors.New("upload staging object is not a GGUF file")
	}
	return nil
}

func uploadStageHasGGUFMagic(
	ctx context.Context,
	options Options,
	reader uploadstagingports.Reader,
) (bool, error) {
	if options.UploadStage == nil {
		return false, errors.New("upload stage handle must not be nil")
	}

	var (
		output uploadstagingports.OpenReadOutput
		err    error
	)
	if rangeReader, ok := reader.(uploadstagingports.RangeReader); ok {
		output, err = rangeReader.OpenReadRange(ctx, uploadstagingports.OpenReadRangeInput{
			Bucket: strings.TrimSpace(options.UploadStage.Bucket),
			Key:    strings.TrimSpace(options.UploadStage.Key),
			Offset: 0,
			Length: 4,
		})
	} else {
		output, err = reader.OpenRead(ctx, uploadstagingports.OpenReadInput{
			Bucket: strings.TrimSpace(options.UploadStage.Bucket),
			Key:    strings.TrimSpace(options.UploadStage.Key),
		})
	}
	if err != nil {
		return false, err
	}
	defer output.Body.Close()

	header := make([]byte, 4)
	n, err := io.ReadFull(output.Body, header)
	switch {
	case err == nil:
		return n == 4 && string(header) == "GGUF", nil
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return false, nil
	default:
		return false, err
	}
}

func uploadStageFileName(options Options) (string, error) {
	if options.UploadStage == nil {
		return "", errors.New("upload stage handle must not be nil")
	}
	fileName := strings.TrimSpace(options.UploadStage.FileName)
	if fileName == "" {
		fileName = filepath.Base(strings.TrimSpace(options.UploadStage.Key))
	}
	fileName = filepath.Base(strings.TrimSpace(fileName))
	switch fileName {
	case "", ".", string(filepath.Separator):
		return "", errors.New("upload stage file name must not be empty")
	default:
		return fileName, nil
	}
}
