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
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/dataplane/publishworker"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
)

const uploadTokenEnv = "AI_MODELS_UPLOAD_TOKEN"

func runUploadSession(args []string) int {
	flags := cmdsupport.NewFlagSet(commandUploadSession)

	var artifactURI string
	var inputFormat string
	var expectedSizeBytes int64
	var task string
	var listenPort int
	var uploadToken string
	var runtimeEngines cmdsupport.RepeatedStringFlag

	flags.StringVar(&artifactURI, "artifact-uri", "", "Controller-owned destination OCI reference.")
	flags.StringVar(&inputFormat, "input-format", "", "Model input format. Leave empty for auto-detection.")
	flags.Int64Var(&expectedSizeBytes, "expected-size-bytes", 0, "Expected upload size in bytes.")
	flags.StringVar(&task, "task", cmdsupport.EnvOr(publishTaskEnv, ""), "Runtime task.")
	flags.IntVar(&listenPort, "listen-port", 8444, "Listen port.")
	flags.StringVar(&uploadToken, "upload-token", cmdsupport.EnvOr(uploadTokenEnv, ""), "Bearer token for upload session.")
	flags.Var(&runtimeEngines, "runtime-engine", "Compatible runtime engine. Repeat the flag for multiple engines.")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	result, err := uploadsessionruntime.Run(ctx, uploadsessionruntime.Options{
		ListenPort:        listenPort,
		UploadToken:       uploadToken,
		ExpectedSizeBytes: expectedSizeBytes,
		InputFormat:       modelsv1alpha1.ModelInputFormat(inputFormat),
		Publish: publishworker.Options{
			ArtifactURI:        artifactURI,
			Task:               task,
			RuntimeEngines:     []string(runtimeEngines),
			ModelPackPublisher: kitops.New(),
			RegistryAuth:       cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv),
		},
	})
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandUploadSession, err)
	}
	if err := cmdsupport.WriteTerminationResult(result); err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandUploadSession, err)
	}

	return 0
}
