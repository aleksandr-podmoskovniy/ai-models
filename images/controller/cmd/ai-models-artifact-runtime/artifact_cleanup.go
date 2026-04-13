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

	"github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/kitops"
	uploadstagings3 "github.com/deckhouse/ai-models/controller/internal/adapters/uploadstaging/s3"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/dataplane/artifactcleanup"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func runArtifactCleanup(args []string) int {
	flags := cmdsupport.NewFlagSet(commandArtifactCleanup)

	var handleJSON string
	var dryRun bool

	flags.StringVar(&handleJSON, "handle-json", "", "Encoded cleanup handle JSON.")
	flags.BoolVar(&dryRun, "dry-run", false, "Validate cleanup handle without removing the artifact.")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	handle, err := cleanuphandle.Decode(handleJSON)
	if err != nil {
		return cmdsupport.CommandError(commandArtifactCleanup, err)
	}

	logger := slog.Default().With(
		slog.String("handleKind", string(handle.Kind)),
		slog.Bool("dryRun", dryRun),
	)
	logger.Info("artifact cleanup started")

	var objectStorageRemover *uploadstagings3.Adapter
	if handle.Kind == cleanuphandle.KindUploadStaging || handle.Kind == cleanuphandle.KindBackendArtifact {
		objectStorageRemover, err = uploadstagings3.New(uploadStagingS3ConfigFromEnv())
		if err != nil {
			return cmdsupport.CommandError(commandArtifactCleanup, err)
		}
	}

	if err := artifactcleanup.Run(ctx, artifactcleanup.Options{
		HandleJSON:          handleJSON,
		DryRun:              dryRun,
		Remover:             kitops.New(),
		StagingRemover:      objectStorageRemover,
		PrefixRemover:       objectStorageRemover,
		ObjectStorageBucket: cmdsupport.EnvOr("AI_MODELS_S3_BUCKET", ""),
		RegistryAuth:        cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv),
	}); err != nil {
		logger.Error("artifact cleanup failed", slog.Any("error", err))
		return 1
	}

	logger.Info("artifact cleanup completed")
	return 0
}
