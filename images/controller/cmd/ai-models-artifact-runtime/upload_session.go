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
	"github.com/deckhouse/ai-models/controller/internal/adapters/uploadstaging/s3"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

const uploadTokenEnv = "AI_MODELS_UPLOAD_TOKEN"

func runUploadSession(args []string) int {
	flags := cmdsupport.NewFlagSet(commandUploadSession)

	var expectedSizeBytes int64
	var listenPort int
	var stagingBucket string
	var stagingKeyPrefix string
	var uploadToken string

	flags.Int64Var(&expectedSizeBytes, "expected-size-bytes", 0, "Expected upload size in bytes.")
	flags.IntVar(&listenPort, "listen-port", 8444, "Listen port.")
	flags.StringVar(&stagingBucket, "staging-bucket", "", "Bucket used for staged uploads.")
	flags.StringVar(&stagingKeyPrefix, "staging-key-prefix", "", "Object key prefix used for staged uploads.")
	flags.StringVar(&uploadToken, "upload-token", cmdsupport.EnvOr(uploadTokenEnv, ""), "Bearer token for upload session.")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	stagingUploader, err := s3.New(uploadStagingS3ConfigFromEnv())
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandUploadSession, err)
	}

	result, err := uploadsessionruntime.Run(ctx, uploadsessionruntime.Options{
		ListenPort:        listenPort,
		UploadToken:       uploadToken,
		ExpectedSizeBytes: expectedSizeBytes,
		StagingBucket:     stagingBucket,
		StagingKeyPrefix:  stagingKeyPrefix,
		StagingUploader:   stagingUploader,
	})
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandUploadSession, err)
	}
	payload, err := cleanuphandle.Encode(result)
	if err != nil {
		cmdsupport.WriteTerminationFailure(err.Error())
		return cmdsupport.CommandError(commandUploadSession, err)
	}
	cmdsupport.WriteTerminationMessage(payload)
	return 0
}
