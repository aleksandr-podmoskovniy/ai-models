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

package cmdsupport

import uploadstagings3 "github.com/deckhouse/ai-models/controller/internal/adapters/uploadstaging/s3"

func UploadStagingS3ConfigFromEnv() uploadstagings3.Config {
	return uploadstagings3.Config{
		EndpointURL:     EnvOr("AI_MODELS_S3_ENDPOINT_URL", ""),
		Region:          FallbackString(EnvOr("AI_MODELS_S3_REGION", ""), EnvOr("AWS_REGION", "")),
		AccessKeyID:     FallbackString(EnvOr("AWS_ACCESS_KEY_ID", ""), EnvOr("AI_MODELS_AWS_ACCESS_KEY_ID", "")),
		SecretAccessKey: FallbackString(EnvOr("AWS_SECRET_ACCESS_KEY", ""), EnvOr("AI_MODELS_AWS_SECRET_ACCESS_KEY", "")),
		UsePathStyle:    EnvOrBool("AI_MODELS_S3_USE_PATH_STYLE", false),
		Insecure:        EnvOrBool("AI_MODELS_S3_IGNORE_TLS", false),
		CAFile:          EnvOr("AI_MODELS_S3_CA_FILE", ""),
	}
}
