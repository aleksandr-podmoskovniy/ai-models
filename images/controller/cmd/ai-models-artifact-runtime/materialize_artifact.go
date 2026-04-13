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

	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const (
	materializeArtifactURIEnv    = "AI_MODELS_MATERIALIZE_ARTIFACT_URI"
	materializeArtifactDigestEnv = "AI_MODELS_MATERIALIZE_ARTIFACT_DIGEST"
	materializeDestinationEnv    = "AI_MODELS_MATERIALIZE_DESTINATION_DIR"
	materializeArtifactFamilyEnv = "AI_MODELS_MATERIALIZE_ARTIFACT_FAMILY"
)

func runMaterializeArtifact(args []string) int {
	flags := cmdsupport.NewFlagSet(commandMaterializeArtifact)

	var artifactURI string
	var artifactDigest string
	var destinationDir string
	var artifactFamily string

	flags.StringVar(&artifactURI, "artifact-uri", cmdsupport.EnvOr(materializeArtifactURIEnv, ""), "Immutable published OCI reference.")
	flags.StringVar(&artifactDigest, "artifact-digest", cmdsupport.EnvOr(materializeArtifactDigestEnv, ""), "Expected immutable digest.")
	flags.StringVar(&destinationDir, "destination-dir", cmdsupport.EnvOr(materializeDestinationEnv, ""), "Destination directory for local model materialization.")
	flags.StringVar(&artifactFamily, "artifact-family", cmdsupport.EnvOr(materializeArtifactFamilyEnv, ""), "Optional canonical artifact family label.")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	logger := slog.Default().With(
		slog.String("artifactURI", artifactURI),
		slog.String("artifactDigest", artifactDigest),
		slog.String("destinationDir", destinationDir),
	)
	if artifactFamily != "" {
		logger = logger.With(slog.String("artifactFamily", artifactFamily))
	}
	logger.Info("artifact materialization started")

	result, err := modelpackoci.NewMaterializer().Materialize(ctx, modelpack.MaterializeInput{
		ArtifactURI:    artifactURI,
		ArtifactDigest: artifactDigest,
		DestinationDir: destinationDir,
		ArtifactFamily: artifactFamily,
	}, cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv))
	if err != nil {
		logger.Error("artifact materialization failed", slog.Any("error", err))
		return cmdsupport.CommandError(commandMaterializeArtifact, err)
	}

	logger.Info(
		"artifact materialization completed",
		slog.String("digest", result.Digest),
		slog.String("mediaType", result.MediaType),
		slog.String("modelPath", result.ModelPath),
		slog.String("markerPath", result.MarkerPath),
	)

	return 0
}
