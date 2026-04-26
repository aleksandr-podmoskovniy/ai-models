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
	"os"
	"path/filepath"
	"strings"
	"time"

	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const (
	materializeArtifactURIEnv    = "AI_MODELS_MATERIALIZE_ARTIFACT_URI"
	materializeArtifactDigestEnv = "AI_MODELS_MATERIALIZE_ARTIFACT_DIGEST"
	materializeDestinationEnv    = "AI_MODELS_MATERIALIZE_DESTINATION_DIR"
	materializeCacheRootEnv      = "AI_MODELS_MATERIALIZE_CACHE_ROOT"
	materializeArtifactFamilyEnv = "AI_MODELS_MATERIALIZE_ARTIFACT_FAMILY"
	materializeSharedStoreEnv    = "AI_MODELS_MATERIALIZE_SHARED_STORE"
	materializeModelAliasEnv     = "AI_MODELS_MATERIALIZE_MODEL_ALIAS"
)

func runMaterializeArtifact(args []string) int {
	flags := cmdsupport.NewFlagSet(commandMaterializeArtifact)

	var artifactURI string
	var artifactDigest string
	var destinationDir string
	var cacheRoot string
	var artifactFamily string
	var modelAlias string

	flags.StringVar(&artifactURI, "artifact-uri", cmdsupport.EnvOr(materializeArtifactURIEnv, ""), "Immutable published OCI reference.")
	flags.StringVar(&artifactDigest, "artifact-digest", cmdsupport.EnvOr(materializeArtifactDigestEnv, ""), "Expected immutable digest.")
	flags.StringVar(&destinationDir, "destination-dir", cmdsupport.EnvOr(materializeDestinationEnv, ""), "Destination directory for local model materialization.")
	flags.StringVar(&cacheRoot, "cache-root", cmdsupport.EnvOr(materializeCacheRootEnv, ""), "Model cache root with stable current symlink.")
	flags.StringVar(&artifactFamily, "artifact-family", cmdsupport.EnvOr(materializeArtifactFamilyEnv, ""), "Optional canonical artifact family label.")
	flags.StringVar(&modelAlias, "model-alias", cmdsupport.EnvOr(materializeModelAliasEnv, ""), "Optional stable workload model alias link.")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	destinationDir, cacheCurrent, err := resolveMaterializationPaths(artifactURI, artifactDigest, destinationDir, cacheRoot)
	if err != nil {
		return cmdsupport.CommandError(commandMaterializeArtifact, err)
	}
	coordination, err := resolveMaterializeCoordination(cacheRoot)
	if err != nil {
		return cmdsupport.CommandError(commandMaterializeArtifact, err)
	}
	if cacheCurrent != "" {
		if err := os.MkdirAll(filepath.Dir(destinationDir), 0o755); err != nil {
			return cmdsupport.CommandError(commandMaterializeArtifact, err)
		}
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
	if modelAlias != "" {
		logger = logger.With(slog.String("modelAlias", modelAlias))
	}
	if cacheCurrent != "" {
		logger = logger.With(slog.String("cacheRoot", cacheRoot))
	}
	if coordination.Mode != "" {
		logger = logger.With(
			slog.String("coordinationMode", coordination.Mode),
			slog.String("coordinationHolder", coordination.HolderID),
		)
	}
	if materializeUsesSharedStore() {
		logger = logger.With(slog.Bool("sharedStore", true))
	}
	cmdsupport.SetDefaultLogger(logger)
	logger.Info("artifact materialization started")

	input := modelpack.MaterializeInput{
		ArtifactURI:    artifactURI,
		ArtifactDigest: artifactDigest,
		DestinationDir: destinationDir,
		ArtifactFamily: artifactFamily,
	}
	auth := cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv)
	result, err := nodecache.MaterializeWithCoordination(ctx, cacheRoot, destinationDir, coordination, func(ctx context.Context) (modelpack.MaterializeResult, error) {
		return modelpackoci.NewMaterializer().Materialize(ctx, input, auth)
	})
	if err != nil {
		logger.Error("artifact materialization failed", slog.Any("error", err))
		return cmdsupport.CommandError(commandMaterializeArtifact, err)
	}
	if modelAlias != "" {
		if cacheRoot == "" {
			err := errors.New("model-alias requires cache-root")
			logger.Error("artifact materialization failed", slog.Any("error", err))
			return cmdsupport.CommandError(commandMaterializeArtifact, err)
		}
		aliasTarget := result.ModelPath
		if strings.TrimSpace(result.Digest) != "" {
			aliasTarget = nodecache.SharedArtifactModelPath(cacheRoot, result.Digest)
		}
		if err := nodecache.UpdateWorkloadModelAliasLink(cacheRoot, modelAlias, aliasTarget); err != nil {
			logger.Error("artifact materialization failed", slog.Any("error", err))
			return cmdsupport.CommandError(commandMaterializeArtifact, err)
		}
	}
	if cacheCurrent != "" && !materializeUsesSharedStore() && coordination.Mode == "" {
		if err := nodecache.UpdateCurrentLink(cacheRoot, result.ModelPath); err != nil {
			logger.Error("artifact materialization failed", slog.Any("error", err))
			return cmdsupport.CommandError(commandMaterializeArtifact, err)
		}
		if err := nodecache.TouchUsage(destinationDir, time.Time{}); err != nil {
			logger.Error("artifact materialization failed", slog.Any("error", err))
			return cmdsupport.CommandError(commandMaterializeArtifact, err)
		}
	}
	if cacheCurrent != "" && !materializeUsesSharedStore() && coordination.Mode == "" {
		if err := nodecache.UpdateWorkloadModelLink(cacheRoot); err != nil {
			logger.Error("artifact materialization failed", slog.Any("error", err))
			return cmdsupport.CommandError(commandMaterializeArtifact, err)
		}
		result.ModelPath = nodecache.WorkloadModelPath(cacheRoot)
	} else if cacheRoot != "" {
		result.ModelPath = nodecache.SharedArtifactModelPath(cacheRoot, result.Digest)
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

func resolveMaterializationPaths(artifactURI, artifactDigest, destinationDir, cacheRoot string) (string, string, error) {
	destinationDir = strings.TrimSpace(destinationDir)
	cacheRoot = strings.TrimSpace(cacheRoot)
	switch {
	case destinationDir != "" && cacheRoot != "":
		return "", "", errors.New("destination-dir and cache-root are mutually exclusive")
	case cacheRoot == "":
		if destinationDir == "" {
			return "", "", errors.New("either destination-dir or cache-root must be set")
		}
		return destinationDir, "", nil
	}
	layout, err := nodecache.ResolveMaterializationLayout(cacheRoot, artifactURI, artifactDigest)
	if err != nil {
		return "", "", err
	}
	return layout.DestinationDir, layout.CurrentLinkPath, nil
}

func materializeUsesSharedStore() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(materializeSharedStoreEnv)), "true")
}
