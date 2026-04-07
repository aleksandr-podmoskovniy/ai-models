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

import "github.com/deckhouse/ai-models/controller/internal/cmdsupport"

const (
	commandPublishWorker   = "publish-worker"
	commandUploadSession   = "upload-session"
	commandArtifactCleanup = "artifact-cleanup"
)

func run(args []string) int {
	switch {
	case len(args) == 0:
		return cmdsupport.CommandError("ai-models-artifact-runtime", errMissingCommand)
	case args[0] == commandPublishWorker:
		return runPublishWorker(args[1:])
	case args[0] == commandUploadSession:
		return runUploadSession(args[1:])
	case args[0] == commandArtifactCleanup:
		return runArtifactCleanup(args[1:])
	default:
		return cmdsupport.CommandError("ai-models-artifact-runtime", errMissingCommand)
	}
}
