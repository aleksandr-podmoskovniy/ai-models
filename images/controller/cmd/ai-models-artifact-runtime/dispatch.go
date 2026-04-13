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
	"errors"

	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
)

const (
	logFormatEnv               = "LOG_FORMAT"
	publicationOCIInsecureEnv  = "PUBLICATION_OCI_INSECURE"
	commandPublishWorker       = "publish-worker"
	commandUploadSession       = "upload-session"
	commandUploadGateway       = "upload-gateway"
	commandArtifactCleanup     = "artifact-cleanup"
	commandMaterializeArtifact = "materialize-artifact"
)

var errMissingCommand = errors.New("expected one of: publish-worker, upload-gateway, artifact-cleanup, materialize-artifact")

func run(args []string) int {
	if err := configureRuntimeLogger(runtimeComponent(args)); err != nil {
		return cmdsupport.CommandError("ai-models-artifact-runtime", err)
	}

	switch {
	case len(args) == 0:
		return cmdsupport.CommandError("ai-models-artifact-runtime", errMissingCommand)
	case args[0] == commandPublishWorker:
		return runPublishWorker(args[1:])
	case args[0] == commandUploadSession || args[0] == commandUploadGateway:
		return runUploadSession(args[1:])
	case args[0] == commandArtifactCleanup:
		return runArtifactCleanup(args[1:])
	case args[0] == commandMaterializeArtifact:
		return runMaterializeArtifact(args[1:])
	default:
		return cmdsupport.CommandError("ai-models-artifact-runtime", errMissingCommand)
	}
}

func configureRuntimeLogger(component string) error {
	logger, err := cmdsupport.NewComponentLogger(cmdsupport.EnvOr(logFormatEnv, "text"), component)
	if err != nil {
		return err
	}
	cmdsupport.SetDefaultLogger(logger)
	return nil
}

func runtimeComponent(args []string) string {
	if len(args) == 0 {
		return "artifact-runtime"
	}

	switch args[0] {
	case commandPublishWorker:
		return "publish-worker"
	case commandUploadSession, commandUploadGateway:
		return "upload-gateway"
	case commandArtifactCleanup:
		return "artifact-cleanup"
	case commandMaterializeArtifact:
		return "materialize-artifact"
	default:
		return "artifact-runtime"
	}
}
