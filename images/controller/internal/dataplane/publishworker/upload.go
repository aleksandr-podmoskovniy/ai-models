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
	"os"
	"path/filepath"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
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

	workspace, cleanupDir, err := ensureWorkspace(options.SnapshotDir, "ai-model-upload-publish-")
	if err != nil {
		return publicationartifact.Result{}, err
	}
	defer cleanupDir()
	logger.Debug("upload publication workspace prepared", slog.String("workspace", workspace))

	sourcePrepareStarted := time.Now()
	logger.Info("upload source preparation started")
	uploadPath, cleanupUpload, err := ensureUploadPath(ctx, options, workspace)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	defer cleanupUpload()
	if info, statErr := os.Stat(uploadPath); statErr == nil {
		logger.Info(
			"upload source preparation completed",
			slog.Int64("durationMs", time.Since(sourcePrepareStarted).Milliseconds()),
			slog.String("uploadPath", uploadPath),
			slog.Int64("uploadSizeBytes", info.Size()),
		)
	}

	checkpointPrepareStarted := time.Now()
	logger.Info("upload checkpoint materialization started")
	checkpointDir, err := sourcefetch.PrepareModelInput(uploadPath, filepath.Join(workspace, "checkpoint"))
	if err != nil {
		return publicationartifact.Result{}, err
	}
	logger.Info(
		"upload checkpoint materialization completed",
		slog.Int64("durationMs", time.Since(checkpointPrepareStarted).Milliseconds()),
		slog.String("checkpointDir", checkpointDir),
	)

	inputFormat, err := resolveUploadInputFormat(checkpointDir, options.InputFormat)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	logger.Info("upload input format resolved", slog.String("resolvedInputFormat", strings.TrimSpace(string(inputFormat))))

	validationStarted := time.Now()
	logger.Info("upload checkpoint validation started")
	if err := modelformat.ValidateDir(checkpointDir, inputFormat); err != nil {
		return publicationartifact.Result{}, err
	}
	logger.Info(
		"upload checkpoint validation completed",
		slog.Int64("durationMs", time.Since(validationStarted).Milliseconds()),
	)

	resolvedProfile, publishResult, err := resolveAndPublish(ctx, options, checkpointDir, inputFormat, sourceProfileInput{
		Task: options.Task,
	})
	if err != nil {
		return publicationartifact.Result{}, err
	}

	if options.UploadStage != nil {
		cleanupStarted := time.Now()
		logger.Info("upload staging cleanup started")
		if err := cleanupUploadStage(ctx, options); err != nil {
			return publicationartifact.Result{}, err
		}
		logger.Info(
			"upload staging cleanup completed",
			slog.Int64("durationMs", time.Since(cleanupStarted).Milliseconds()),
		)
	}

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
	), nil
}

func ensureUploadPath(ctx context.Context, options Options, workspace string) (string, func(), error) {
	if strings.TrimSpace(options.UploadPath) != "" {
		return options.UploadPath, func() {}, nil
	}
	if options.UploadStage == nil {
		return "", nil, errors.New("upload staging handle must not be empty")
	}
	if options.UploadStaging == nil {
		return "", nil, errors.New("upload staging client must not be nil")
	}

	fileName := strings.TrimSpace(options.UploadStage.FileName)
	if fileName == "" {
		fileName = "upload.bin"
	}
	localPath := filepath.Join(workspace, fileName)
	if err := options.UploadStaging.Download(ctx, uploadstagingports.DownloadInput{
		Bucket:          options.UploadStage.Bucket,
		Key:             options.UploadStage.Key,
		DestinationPath: localPath,
	}); err != nil {
		return "", nil, err
	}
	return localPath, func() {
		_ = os.Remove(localPath)
	}, nil
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
