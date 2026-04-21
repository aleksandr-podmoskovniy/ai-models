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
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func publishFromUpload(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.UploadPath) == "" && options.UploadStage == nil {
		return publicationartifact.Result{}, errors.New("upload source requires either upload-path or upload staging handle")
	}
	logger := slog.Default().With(
		slog.String("sourceType", string(modelsv1alpha1.ModelSourceTypeUpload)),
		slog.Bool("localUploadPathProvided", strings.TrimSpace(options.UploadPath) != ""),
		slog.Bool("uploadStageEnabled", options.UploadStage != nil),
	)

	publishResult, handled, err := tryPublishUploadFastPaths(ctx, options, logger)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	if handled {
		return publishResult, nil
	}
	if options.UploadStage != nil {
		return publicationartifact.Result{}, errors.New("staged upload path escaped zero-copy publish fast paths after validation")
	}
	return publicationartifact.Result{}, errors.New("upload path escaped zero-copy publish fast paths after validation")
}

func tryPublishUploadFastPaths(
	ctx context.Context,
	options Options,
	logger *slog.Logger,
) (publicationartifact.Result, bool, error) {
	publishResult, handled, err := tryPublishUploadStageDirectObjectSource(ctx, options, logger)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if handled {
		return publishResult, true, nil
	}

	publishResult, handled, err = tryPublishUploadStageStreamingArchive(ctx, options, logger)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if handled {
		return publishResult, true, nil
	}

	if err := failFastUploadProbe(ctx, options, logger); err != nil {
		return publicationartifact.Result{}, false, err
	}

	publishResult, handled, err = tryPublishLocalZeroCopyUpload(ctx, options, logger)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if handled {
		return publishResult, true, nil
	}
	if strings.TrimSpace(options.UploadPath) != "" {
		return publicationartifact.Result{}, false, errors.New("local upload path escaped zero-copy publish fast paths after fail-fast validation")
	}
	return publicationartifact.Result{}, false, nil
}

func tryPublishLocalZeroCopyUpload(
	ctx context.Context,
	options Options,
	logger *slog.Logger,
) (publicationartifact.Result, bool, error) {
	uploadPath := strings.TrimSpace(options.UploadPath)
	if uploadPath == "" {
		return publicationartifact.Result{}, false, nil
	}

	publishResult, handled, err := tryPublishDirectUpload(ctx, options, uploadPath, logger)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if handled {
		return publishResult, true, nil
	}

	publishResult, handled, err = tryPublishStreamingUploadArchive(ctx, options, uploadPath, logger)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if handled {
		return publishResult, true, nil
	}

	return publicationartifact.Result{}, false, nil
}

func tryPublishDirectUpload(
	ctx context.Context,
	options Options,
	uploadPath string,
	logger *slog.Logger,
) (publicationartifact.Result, bool, error) {
	directInputFormat, err := resolveDirectUploadInputFormat(uploadPath, options.InputFormat)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if directInputFormat != modelsv1alpha1.ModelInputFormatGGUF {
		return publicationartifact.Result{}, false, nil
	}

	logger.Info(
		"upload direct model path selected",
		slog.String("uploadPath", uploadPath),
		slog.String("resolvedInputFormat", string(directInputFormat)),
	)
	if err := modelformat.ValidatePath(uploadPath, directInputFormat); err != nil {
		return publicationartifact.Result{}, false, err
	}
	resolvedProfile, publishResult, err := resolveAndPublishWithLayers(ctx, options, uploadPath, directInputFormat, sourceProfileInput{
		Task: options.Task,
	}, []modelpackports.PublishLayer{
		{
			SourcePath:  uploadPath,
			TargetPath:  filepath.Base(uploadPath),
			Base:        modelpackports.LayerBaseModel,
			Format:      modelpackports.LayerFormatRaw,
			Compression: modelpackports.LayerCompressionNone,
		},
	}, nil)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if err := cleanupStagedUploadObject(ctx, options, logger); err != nil {
		return publicationartifact.Result{}, false, err
	}

	return buildUploadResult(options, resolvedProfile, publishResult), true, nil
}

func cleanupStagedUploadObject(ctx context.Context, options Options, logger *slog.Logger) error {
	if options.UploadStage == nil {
		return nil
	}
	cleanupStarted := time.Now()
	logger.Info("upload staging cleanup started")
	if err := cleanupUploadStage(ctx, options); err != nil {
		return err
	}
	logger.Info(
		"upload staging cleanup completed",
		slog.Int64("durationMs", time.Since(cleanupStarted).Milliseconds()),
	)
	return nil
}

func cleanupUploadStage(ctx context.Context, options Options) error {
	if options.UploadStage == nil {
		return nil
	}
	if options.UploadStaging == nil {
		return errors.New("upload staging client must not be nil")
	}
	return options.UploadStaging.Delete(ctx, uploadstagingports.DeleteInput{
		Bucket: options.UploadStage.Bucket,
		Key:    options.UploadStage.Key,
	})
}

func buildUploadResult(
	options Options,
	resolvedProfile publicationdata.ResolvedProfile,
	publishResult modelpackports.PublishResult,
) publicationartifact.Result {
	rawSource := uploadRawProvenance(options.UploadStage)
	return buildBackendResult(
		publicationdata.SourceProvenance{
			Type:           modelsv1alpha1.ModelSourceTypeUpload,
			RawURI:         rawSource.RawURI,
			RawObjectCount: rawSource.RawObjectCount,
			RawSizeBytes:   rawSource.RawSizeBytes,
		},
		resolvedProfile,
		publishResult,
	)
}

func resolveDirectUploadInputFormat(uploadPath string, requested modelsv1alpha1.ModelInputFormat) (modelsv1alpha1.ModelInputFormat, error) {
	if isArchiveUploadPath(uploadPath) {
		return "", nil
	}
	return resolveUploadInputFormat(uploadPath, requested)
}

func isArchiveUploadPath(uploadPath string) bool {
	lowerPath := strings.ToLower(strings.TrimSpace(uploadPath))
	return strings.HasSuffix(lowerPath, ".tar") ||
		strings.HasSuffix(lowerPath, ".tar.gz") ||
		strings.HasSuffix(lowerPath, ".tgz") ||
		strings.HasSuffix(lowerPath, ".tar.zst") ||
		strings.HasSuffix(lowerPath, ".tar.zstd") ||
		strings.HasSuffix(lowerPath, ".tzst") ||
		strings.HasSuffix(lowerPath, ".zip")
}
