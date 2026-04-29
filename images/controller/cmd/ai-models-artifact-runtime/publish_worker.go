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

package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	directuploadstate "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/directuploadstate"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageaccounting"
	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	uploadstagings3 "github.com/deckhouse/ai-models/controller/internal/adapters/uploadstaging/s3"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/dataplane/publishworker"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

const (
	publishSourceTypeEnv          = "AI_MODELS_PUBLISH_SOURCE_TYPE"
	publishSourceURLEnv           = "AI_MODELS_IMPORT_SOURCE_URL"
	publishHFModelIDEnv           = "AI_MODELS_IMPORT_HF_MODEL_ID"
	publishUploadPathEnv          = "AI_MODELS_IMPORT_UPLOAD_PATH"
	publishUploadStageBucketEnv   = "AI_MODELS_IMPORT_UPLOAD_STAGE_BUCKET"
	publishUploadStageKeyEnv      = "AI_MODELS_IMPORT_UPLOAD_STAGE_KEY"
	publishUploadStageFileNameEnv = "AI_MODELS_IMPORT_UPLOAD_STAGE_FILE_NAME"
	publishRawStageBucketEnv      = "AI_MODELS_IMPORT_RAW_STAGE_BUCKET"
	publishRawStageKeyPrefixEnv   = "AI_MODELS_IMPORT_RAW_STAGE_KEY_PREFIX"
	publishOCIDirectUploadEnv     = "AI_MODELS_IMPORT_OCI_DIRECT_UPLOAD_ENDPOINT"
	publishSourceFetchEnv         = "AI_MODELS_IMPORT_SOURCE_FETCH_MODE"
	publishInputFormatEnv         = "AI_MODELS_IMPORT_INPUT_FORMAT"
	publishRevisionEnv            = "AI_MODELS_IMPORT_HF_REVISION"
	publishTaskEnv                = "AI_MODELS_IMPORT_TASK"
	publishDirectUploadStateNS    = "AI_MODELS_DIRECT_UPLOAD_STATE_NAMESPACE"
	publishDirectUploadStateName  = "AI_MODELS_DIRECT_UPLOAD_STATE_SECRET_NAME"
)

func runPublishWorker(args []string) int {
	flags := cmdsupport.NewFlagSet(commandPublishWorker)

	var sourceType string
	var artifactURI string
	var sourceURL string
	var hfModelID string
	var uploadPath string
	var uploadStageBucket string
	var uploadStageKey string
	var uploadStageFileName string
	var rawStageBucket string
	var rawStageKeyPrefix string
	var ociDirectUploadEndpoint string
	var sourceFetchMode string
	var inputFormat string
	var revision string
	var task string
	var directUploadStateNamespace string
	var directUploadStateName string
	var storageAccountingNamespace string
	var storageAccountingSecretName string
	var storageCapacityLimit string
	var storageReservationID string
	var storageOwnerKind string
	var storageOwnerName string
	var storageOwnerNamespace string
	var storageOwnerUID string

	flags.StringVar(&sourceType, "source-type", cmdsupport.EnvOr(publishSourceTypeEnv, string(modelsv1alpha1.ModelSourceTypeHuggingFace)), "Source type: HuggingFace, Ollama or Upload.")
	flags.StringVar(&artifactURI, "artifact-uri", "", "Controller-owned destination OCI reference.")
	flags.StringVar(&sourceURL, "source-url", cmdsupport.EnvOr(publishSourceURLEnv, ""), "Remote source URL for non-HuggingFace providers.")
	flags.StringVar(&hfModelID, "hf-model-id", cmdsupport.EnvOr(publishHFModelIDEnv, ""), "Hugging Face repo ID.")
	flags.StringVar(&uploadPath, "upload-path", cmdsupport.EnvOr(publishUploadPathEnv, ""), "Uploaded archive path.")
	flags.StringVar(&uploadStageBucket, "upload-stage-bucket", cmdsupport.EnvOr(publishUploadStageBucketEnv, ""), "Bucket containing staged upload input.")
	flags.StringVar(&uploadStageKey, "upload-stage-key", cmdsupport.EnvOr(publishUploadStageKeyEnv, ""), "Object key containing staged upload input.")
	flags.StringVar(&uploadStageFileName, "upload-stage-file-name", cmdsupport.EnvOr(publishUploadStageFileNameEnv, ""), "Original staged upload file name.")
	flags.StringVar(&rawStageBucket, "raw-stage-bucket", cmdsupport.EnvOr(publishRawStageBucketEnv, ""), "Bucket used for controller-owned raw staging of remote sources.")
	flags.StringVar(&rawStageKeyPrefix, "raw-stage-key-prefix", cmdsupport.EnvOr(publishRawStageKeyPrefixEnv, ""), "Object key prefix used for controller-owned raw staging of remote sources.")
	flags.StringVar(&ociDirectUploadEndpoint, "oci-direct-upload-endpoint", cmdsupport.EnvOr(publishOCIDirectUploadEnv, ""), "Internal DMCR direct-upload HTTPS endpoint used to stream published blob payloads into backing storage.")
	flags.StringVar(&sourceFetchMode, "source-fetch-mode", cmdsupport.EnvOr(publishSourceFetchEnv, string(publicationports.SourceFetchModeDirect)), "Remote source fetch mode: mirror or direct.")
	flags.StringVar(&inputFormat, "input-format", cmdsupport.EnvOr(publishInputFormatEnv, ""), "Model input format. Leave empty for auto-detection.")
	flags.StringVar(&revision, "revision", cmdsupport.EnvOr(publishRevisionEnv, ""), "Resolved source revision.")
	flags.StringVar(&task, "task", cmdsupport.EnvOr(publishTaskEnv, ""), "Runtime task.")
	flags.StringVar(&directUploadStateNamespace, "direct-upload-state-namespace", cmdsupport.EnvOr(publishDirectUploadStateNS, ""), "Namespace of the direct-upload state Secret.")
	flags.StringVar(&directUploadStateName, "direct-upload-state-secret-name", cmdsupport.EnvOr(publishDirectUploadStateName, ""), "Name of the direct-upload state Secret.")
	flags.StringVar(&storageAccountingNamespace, "storage-accounting-namespace", "", "Namespace of the storage accounting Secret.")
	flags.StringVar(&storageAccountingSecretName, "storage-accounting-secret-name", storageaccounting.DefaultSecretName, "Storage accounting Secret name.")
	flags.StringVar(&storageCapacityLimit, "storage-capacity-limit", "", "Optional total artifact storage capacity limit.")
	flags.StringVar(&storageReservationID, "storage-reservation-id", "", "Stable storage reservation ID for this publication.")
	flags.StringVar(&storageOwnerKind, "storage-owner-kind", "", "Storage reservation owner kind.")
	flags.StringVar(&storageOwnerName, "storage-owner-name", "", "Storage reservation owner name.")
	flags.StringVar(&storageOwnerNamespace, "storage-owner-namespace", "", "Storage reservation owner namespace.")
	flags.StringVar(&storageOwnerUID, "storage-owner-uid", "", "Storage reservation owner UID.")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	var err error
	var uploadStage *cleanuphandle.UploadStagingHandle
	var uploadStagingClient *uploadstagings3.Adapter
	if uploadStageKey != "" || uploadStageBucket != "" {
		uploadStage = &cleanuphandle.UploadStagingHandle{
			Bucket:   uploadStageBucket,
			Key:      uploadStageKey,
			FileName: uploadStageFileName,
		}
	}
	if uploadStage != nil || rawStageBucket != "" || rawStageKeyPrefix != "" {
		uploadStagingClient, err = uploadstagings3.New(cmdsupport.UploadStagingS3ConfigFromEnv())
		if err != nil {
			cmdsupport.WriteTerminationFailure(err.Error())
			return cmdsupport.CommandError(commandPublishWorker, err)
		}
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	var directUploadStateStore modelpackports.DirectUploadStateStore
	if strings.TrimSpace(directUploadStateName) != "" {
		directUploadStateStore, err = directuploadstate.NewInCluster(directUploadStateNamespace, directUploadStateName)
		if err != nil {
			cmdsupport.WriteTerminationFailure(err.Error())
			return cmdsupport.CommandError(commandPublishWorker, err)
		}
	}
	storageReservation, err := newPublishStorageReservation(
		storageAccountingNamespace,
		storageAccountingSecretName,
		storageCapacityLimit,
		publishStorageOwner{
			ID:        storageReservationID,
			Kind:      storageOwnerKind,
			Name:      storageOwnerName,
			Namespace: storageOwnerNamespace,
			UID:       storageOwnerUID,
		},
	)
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandPublishWorker, err)
	}

	logger := publishWorkerLogger(
		modelsv1alpha1.ModelSourceType(sourceType),
		artifactURI,
		sourceURL,
		hfModelID,
		uploadStageFileName,
		modelsv1alpha1.ModelInputFormat(inputFormat),
		task,
	)
	cmdsupport.SetDefaultLogger(logger)
	logger.Info(
		"publication worker started",
		slog.Bool("uploadStageEnabled", uploadStage != nil),
		slog.String("sourceFetchMode", strings.TrimSpace(sourceFetchMode)),
		slog.Bool("sourceMirrorEnabled", uploadStagingClient != nil && strings.TrimSpace(rawStageBucket) != "" && strings.TrimSpace(rawStageKeyPrefix) != ""),
	)

	result, err := publishworker.Run(ctx, publishworker.Options{
		SourceType:              modelsv1alpha1.ModelSourceType(sourceType),
		ArtifactURI:             artifactURI,
		SourceURL:               sourceURL,
		HFModelID:               hfModelID,
		OCIDirectUploadEndpoint: ociDirectUploadEndpoint,
		DirectUploadCAFile:      cmdsupport.EnvOr("AI_MODELS_S3_CA_FILE", ""),
		DirectUploadInsecure:    cmdsupport.EnvOrBool("AI_MODELS_S3_IGNORE_TLS", false),
		SourceFetchMode:         publicationports.SourceFetchMode(sourceFetchMode),
		Revision:                revision,
		UploadPath:              uploadPath,
		UploadStage:             uploadStage,
		RawStageBucket:          rawStageBucket,
		RawStageKeyPrefix:       rawStageKeyPrefix,
		InputFormat:             modelsv1alpha1.ModelInputFormat(inputFormat),
		Task:                    task,
		HFToken:                 cmdsupport.EnvOr("HF_TOKEN", cmdsupport.EnvOr("HUGGING_FACE_HUB_TOKEN", "")),
		UploadStaging:           uploadStagingClient,
		StorageReservation:      storageReservation,
		ModelPackPublisher:      modelpackoci.New(),
		RegistryAuth:            cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv),
		DirectUploadState:       directUploadStateStore,
	})
	if err != nil {
		if shouldPersistDirectUploadFailureState(err) {
			if markErr := persistDirectUploadTerminalState(ctx, directUploadStateStore, modelpackports.DirectUploadStatePhaseFailed, err.Error()); markErr != nil {
				err = errors.Join(err, markErr)
			}
		}
		cmdsupport.WriteTerminationFailure(err.Error())
		if shouldPersistDirectUploadFailureState(err) {
			logger.Error("publication worker failed", slog.Any("error", err))
		} else {
			logger.Warn("publication worker interrupted before completion; keeping direct upload state resumable", slog.Any("error", err))
		}
		return 1
	}
	if err := persistDirectUploadTerminalState(ctx, directUploadStateStore, modelpackports.DirectUploadStatePhaseCompleted, ""); err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		logger.Error("publication worker direct upload state finalization failed", slog.Any("error", err))
		return 1
	}
	payload, err := publicationartifact.EncodeResult(result)
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		logger.Error("publication worker result encoding failed", slog.Any("error", err))
		return 1
	}
	cmdsupport.WriteTerminationMessage(payload)
	logger.Info(
		"publication worker completed",
		slog.String("resolvedFormat", strings.TrimSpace(result.Resolved.Format)),
		slog.String("resolvedTask", strings.TrimSpace(result.Resolved.Task)),
		slog.String("artifactDigest", strings.TrimSpace(result.Artifact.Digest)),
		slog.Int64("artifactSizeBytes", result.Artifact.SizeBytes),
	)

	return 0
}

func publishWorkerLogger(
	sourceType modelsv1alpha1.ModelSourceType,
	artifactURI string,
	sourceURL string,
	hfModelID string,
	uploadFileName string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	task string,
) *slog.Logger {
	logger := slog.Default().With(
		slog.String("sourceType", strings.TrimSpace(string(sourceType))),
		slog.String("artifactURI", strings.TrimSpace(artifactURI)),
	)
	if strings.TrimSpace(string(inputFormat)) != "" {
		logger = logger.With(slog.String("requestedInputFormat", strings.TrimSpace(string(inputFormat))))
	}
	if strings.TrimSpace(task) != "" {
		logger = logger.With(slog.String("task", strings.TrimSpace(task)))
	}

	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace:
		if strings.TrimSpace(hfModelID) != "" {
			logger = logger.With(slog.String("sourceRepoID", strings.TrimSpace(hfModelID)))
		}
	case modelsv1alpha1.ModelSourceTypeOllama:
		if strings.TrimSpace(sourceURL) != "" {
			logger = logger.With(slog.String("sourceURL", strings.TrimSpace(sourceURL)))
		}
	case modelsv1alpha1.ModelSourceTypeUpload:
		if strings.TrimSpace(uploadFileName) != "" {
			logger = logger.With(slog.String("fileName", strings.TrimSpace(uploadFileName)))
		}
	}

	return logger
}

func persistDirectUploadTerminalState(
	ctx context.Context,
	store modelpackports.DirectUploadStateStore,
	phase modelpackports.DirectUploadStatePhase,
	message string,
) error {
	if store == nil {
		return nil
	}

	state, found, err := store.Load(ctx)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	state.Phase = phase
	state.FailureMessage = strings.TrimSpace(message)
	return store.Save(ctx, state)
}

func shouldPersistDirectUploadFailureState(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}
