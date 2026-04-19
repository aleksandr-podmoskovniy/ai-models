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
	"log/slog"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	uploadstagings3 "github.com/deckhouse/ai-models/controller/internal/adapters/uploadstaging/s3"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/dataplane/publishworker"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

const (
	publishSourceTypeEnv          = "AI_MODELS_PUBLISH_SOURCE_TYPE"
	publishHFModelIDEnv           = "AI_MODELS_IMPORT_HF_MODEL_ID"
	publishUploadPathEnv          = "AI_MODELS_IMPORT_UPLOAD_PATH"
	publishUploadStageBucketEnv   = "AI_MODELS_IMPORT_UPLOAD_STAGE_BUCKET"
	publishUploadStageKeyEnv      = "AI_MODELS_IMPORT_UPLOAD_STAGE_KEY"
	publishUploadStageFileNameEnv = "AI_MODELS_IMPORT_UPLOAD_STAGE_FILE_NAME"
	publishRawStageBucketEnv      = "AI_MODELS_IMPORT_RAW_STAGE_BUCKET"
	publishRawStageKeyPrefixEnv   = "AI_MODELS_IMPORT_RAW_STAGE_KEY_PREFIX"
	publishOCIDirectUploadEnv     = "AI_MODELS_IMPORT_OCI_DIRECT_UPLOAD_ENDPOINT"
	publishSourceAcquisitionEnv   = "AI_MODELS_IMPORT_SOURCE_ACQUISITION_MODE"
	publishInputFormatEnv         = "AI_MODELS_IMPORT_INPUT_FORMAT"
	publishRevisionEnv            = "AI_MODELS_IMPORT_HF_REVISION"
	publishTaskEnv                = "AI_MODELS_IMPORT_TASK"
)

func runPublishWorker(args []string) int {
	flags := cmdsupport.NewFlagSet(commandPublishWorker)

	var sourceType string
	var artifactURI string
	var hfModelID string
	var uploadPath string
	var uploadStageBucket string
	var uploadStageKey string
	var uploadStageFileName string
	var rawStageBucket string
	var rawStageKeyPrefix string
	var ociDirectUploadEndpoint string
	var sourceAcquisitionMode string
	var inputFormat string
	var revision string
	var task string

	flags.StringVar(&sourceType, "source-type", cmdsupport.EnvOr(publishSourceTypeEnv, string(modelsv1alpha1.ModelSourceTypeHuggingFace)), "Source type: HuggingFace or Upload.")
	flags.StringVar(&artifactURI, "artifact-uri", "", "Controller-owned destination OCI reference.")
	flags.StringVar(&hfModelID, "hf-model-id", cmdsupport.EnvOr(publishHFModelIDEnv, ""), "Hugging Face repo ID.")
	flags.StringVar(&uploadPath, "upload-path", cmdsupport.EnvOr(publishUploadPathEnv, ""), "Uploaded archive path.")
	flags.StringVar(&uploadStageBucket, "upload-stage-bucket", cmdsupport.EnvOr(publishUploadStageBucketEnv, ""), "Bucket containing staged upload input.")
	flags.StringVar(&uploadStageKey, "upload-stage-key", cmdsupport.EnvOr(publishUploadStageKeyEnv, ""), "Object key containing staged upload input.")
	flags.StringVar(&uploadStageFileName, "upload-stage-file-name", cmdsupport.EnvOr(publishUploadStageFileNameEnv, ""), "Original staged upload file name.")
	flags.StringVar(&rawStageBucket, "raw-stage-bucket", cmdsupport.EnvOr(publishRawStageBucketEnv, ""), "Bucket used for controller-owned raw staging of remote sources.")
	flags.StringVar(&rawStageKeyPrefix, "raw-stage-key-prefix", cmdsupport.EnvOr(publishRawStageKeyPrefixEnv, ""), "Object key prefix used for controller-owned raw staging of remote sources.")
	flags.StringVar(&ociDirectUploadEndpoint, "oci-direct-upload-endpoint", cmdsupport.EnvOr(publishOCIDirectUploadEnv, ""), "Internal DMCR direct-upload HTTPS endpoint for heavy layer blob uploads.")
	flags.StringVar(&sourceAcquisitionMode, "source-acquisition-mode", cmdsupport.EnvOr(publishSourceAcquisitionEnv, string(publicationports.SourceAcquisitionModeDirect)), "Source acquisition mode: mirror or direct.")
	flags.StringVar(&inputFormat, "input-format", cmdsupport.EnvOr(publishInputFormatEnv, ""), "Model input format. Leave empty for auto-detection.")
	flags.StringVar(&revision, "revision", cmdsupport.EnvOr(publishRevisionEnv, ""), "Resolved source revision.")
	flags.StringVar(&task, "task", cmdsupport.EnvOr(publishTaskEnv, ""), "Runtime task.")

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
		uploadStagingClient, err = uploadstagings3.New(uploadStagingS3ConfigFromEnv())
		if err != nil {
			cmdsupport.WriteTerminationFailure(err.Error())
			return cmdsupport.CommandError(commandPublishWorker, err)
		}
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	logger := publishWorkerLogger(
		modelsv1alpha1.ModelSourceType(sourceType),
		artifactURI,
		hfModelID,
		uploadStageFileName,
		modelsv1alpha1.ModelInputFormat(inputFormat),
		task,
	)
	cmdsupport.SetDefaultLogger(logger)
	logger.Info(
		"publication worker started",
		slog.Bool("uploadStageEnabled", uploadStage != nil),
		slog.String("sourceAcquisitionMode", strings.TrimSpace(sourceAcquisitionMode)),
		slog.Bool("sourceMirrorEnabled", uploadStagingClient != nil && strings.TrimSpace(rawStageBucket) != "" && strings.TrimSpace(rawStageKeyPrefix) != ""),
	)

	result, err := publishworker.Run(ctx, publishworker.Options{
		SourceType:                 modelsv1alpha1.ModelSourceType(sourceType),
		ArtifactURI:                artifactURI,
		HFModelID:                  hfModelID,
		OCIDirectUploadEndpoint:    ociDirectUploadEndpoint,
		DirectUploadCAFile:         cmdsupport.EnvOr("AI_MODELS_S3_CA_FILE", ""),
		DirectUploadInsecure:       cmdsupport.EnvOrBool("AI_MODELS_S3_IGNORE_TLS", false),
		SourceAcquisitionMode:      publicationports.SourceAcquisitionMode(sourceAcquisitionMode),
		Revision:                   revision,
		UploadPath:                 uploadPath,
		UploadStage:                uploadStage,
		RawStageBucket:             rawStageBucket,
		RawStageKeyPrefix:          rawStageKeyPrefix,
		InputFormat:                modelsv1alpha1.ModelInputFormat(inputFormat),
		Task:                       task,
		HFToken:                    cmdsupport.EnvOr("HF_TOKEN", cmdsupport.EnvOr("HUGGING_FACE_HUB_TOKEN", "")),
		UploadStaging:              uploadStagingClient,
		ModelPackPublisher:         modelpackoci.New(),
		RegistryAuth:               cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv),
	})
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		logger.Error("publication worker failed", slog.Any("error", err))
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
	case modelsv1alpha1.ModelSourceTypeUpload:
		if strings.TrimSpace(uploadFileName) != "" {
			logger = logger.With(slog.String("fileName", strings.TrimSpace(uploadFileName)))
		}
	}

	return logger
}
