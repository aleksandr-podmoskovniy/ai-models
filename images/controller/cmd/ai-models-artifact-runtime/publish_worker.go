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
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/kitops"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/dataplane/publishworker"
)

const (
	publishSourceTypeEnv      = "AI_MODELS_PUBLISH_SOURCE_TYPE"
	publishHFModelIDEnv       = "AI_MODELS_IMPORT_HF_MODEL_ID"
	publishHTTPURLEnv         = "AI_MODELS_IMPORT_HTTP_URL"
	publishHTTPCABundleB64Env = "AI_MODELS_IMPORT_HTTP_CA_BUNDLE_B64"
	publishHTTPAuthDirEnv     = "AI_MODELS_IMPORT_HTTP_AUTH_DIR"
	publishUploadPathEnv      = "AI_MODELS_IMPORT_UPLOAD_PATH"
	publishInputFormatEnv     = "AI_MODELS_IMPORT_INPUT_FORMAT"
	publishRevisionEnv        = "AI_MODELS_IMPORT_HF_REVISION"
	publishTaskEnv            = "AI_MODELS_IMPORT_TASK"
	publishSnapshotDirEnv     = "AI_MODELS_IMPORT_SNAPSHOT_DIR"
)

func runPublishWorker(args []string) int {
	flags := cmdsupport.NewFlagSet(commandPublishWorker)

	var sourceType string
	var artifactURI string
	var hfModelID string
	var httpURL string
	var httpCABundleB64 string
	var httpAuthDir string
	var uploadPath string
	var inputFormat string
	var revision string
	var task string
	var snapshotDir string
	var runtimeEngines cmdsupport.RepeatedStringFlag

	flags.StringVar(&sourceType, "source-type", cmdsupport.EnvOr(publishSourceTypeEnv, string(modelsv1alpha1.ModelSourceTypeHuggingFace)), "Source type: HuggingFace, HTTP or Upload.")
	flags.StringVar(&artifactURI, "artifact-uri", "", "Controller-owned destination OCI reference.")
	flags.StringVar(&hfModelID, "hf-model-id", cmdsupport.EnvOr(publishHFModelIDEnv, ""), "Hugging Face repo ID.")
	flags.StringVar(&httpURL, "http-url", cmdsupport.EnvOr(publishHTTPURLEnv, ""), "HTTP model URL. May point to an archive or a direct GGUF file.")
	flags.StringVar(&httpCABundleB64, "http-ca-bundle-b64", cmdsupport.EnvOr(publishHTTPCABundleB64Env, ""), "Base64-encoded HTTP CA bundle.")
	flags.StringVar(&httpAuthDir, "http-auth-dir", cmdsupport.EnvOr(publishHTTPAuthDirEnv, ""), "HTTP auth directory.")
	flags.StringVar(&uploadPath, "upload-path", cmdsupport.EnvOr(publishUploadPathEnv, ""), "Uploaded archive path.")
	flags.StringVar(&inputFormat, "input-format", cmdsupport.EnvOr(publishInputFormatEnv, ""), "Model input format. Leave empty for auto-detection.")
	flags.StringVar(&revision, "revision", cmdsupport.EnvOr(publishRevisionEnv, ""), "Resolved source revision.")
	flags.StringVar(&task, "task", cmdsupport.EnvOr(publishTaskEnv, ""), "Runtime task.")
	flags.StringVar(&snapshotDir, "snapshot-dir", cmdsupport.EnvOr(publishSnapshotDirEnv, ""), "Optional work directory.")
	flags.Var(&runtimeEngines, "runtime-engine", "Compatible runtime engine. Repeat the flag for multiple engines.")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	caBundle, err := sourcefetch.DecodeInlineCABundle(httpCABundleB64)
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandPublishWorker, err)
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	result, err := publishworker.Run(ctx, publishworker.Options{
		SourceType:         modelsv1alpha1.ModelSourceType(sourceType),
		ArtifactURI:        artifactURI,
		HFModelID:          hfModelID,
		Revision:           revision,
		HTTPURL:            httpURL,
		HTTPCABundle:       caBundle,
		HTTPAuthDir:        httpAuthDir,
		UploadPath:         uploadPath,
		InputFormat:        modelsv1alpha1.ModelInputFormat(inputFormat),
		Task:               task,
		RuntimeEngines:     []string(runtimeEngines),
		SnapshotDir:        snapshotDir,
		HFToken:            cmdsupport.EnvOr("HF_TOKEN", cmdsupport.EnvOr("HUGGING_FACE_HUB_TOKEN", "")),
		ModelPackPublisher: kitops.New(),
		RegistryAuth:       cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv),
	})
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandPublishWorker, err)
	}
	if err := cmdsupport.WriteTerminationResult(result); err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandPublishWorker, err)
	}

	return 0
}
