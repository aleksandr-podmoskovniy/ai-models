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

	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const (
	materializeArtifactURIEnv    = "AI_MODELS_MATERIALIZE_ARTIFACT_URI"
	materializeArtifactDigestEnv = "AI_MODELS_MATERIALIZE_ARTIFACT_DIGEST"
	materializeDestinationEnv    = "AI_MODELS_MATERIALIZE_DESTINATION_DIR"
	materializeCacheRootEnv      = "AI_MODELS_MATERIALIZE_CACHE_ROOT"
	materializeArtifactFamilyEnv = "AI_MODELS_MATERIALIZE_ARTIFACT_FAMILY"

	cacheStoreDirName = "store"
	cacheCurrentPath  = "current"
)

func runMaterializeArtifact(args []string) int {
	flags := cmdsupport.NewFlagSet(commandMaterializeArtifact)

	var artifactURI string
	var artifactDigest string
	var destinationDir string
	var cacheRoot string
	var artifactFamily string

	flags.StringVar(&artifactURI, "artifact-uri", cmdsupport.EnvOr(materializeArtifactURIEnv, ""), "Immutable published OCI reference.")
	flags.StringVar(&artifactDigest, "artifact-digest", cmdsupport.EnvOr(materializeArtifactDigestEnv, ""), "Expected immutable digest.")
	flags.StringVar(&destinationDir, "destination-dir", cmdsupport.EnvOr(materializeDestinationEnv, ""), "Destination directory for local model materialization.")
	flags.StringVar(&cacheRoot, "cache-root", cmdsupport.EnvOr(materializeCacheRootEnv, ""), "Model cache root with stable current symlink.")
	flags.StringVar(&artifactFamily, "artifact-family", cmdsupport.EnvOr(materializeArtifactFamilyEnv, ""), "Optional canonical artifact family label.")

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
	if cacheCurrent != "" {
		logger = logger.With(slog.String("cacheRoot", cacheRoot))
	}
	if coordination.Mode != "" {
		logger = logger.With(
			slog.String("coordinationMode", coordination.Mode),
			slog.String("coordinationHolder", coordination.HolderID),
		)
	}
	logger.Info("artifact materialization started")

	input := modelpack.MaterializeInput{
		ArtifactURI:    artifactURI,
		ArtifactDigest: artifactDigest,
		DestinationDir: destinationDir,
		ArtifactFamily: artifactFamily,
	}
	auth := cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv)
	result, err := materializeWithCoordination(ctx, cacheRoot, destinationDir, coordination, func(ctx context.Context) (modelpack.MaterializeResult, error) {
		return modelpackoci.NewMaterializer().Materialize(ctx, input, auth)
	})
	if err != nil {
		logger.Error("artifact materialization failed", slog.Any("error", err))
		return cmdsupport.CommandError(commandMaterializeArtifact, err)
	}
	if cacheCurrent != "" && coordination.Mode == "" {
		if err := updateCurrentMaterializationLink(cacheRoot, result.ModelPath); err != nil {
			logger.Error("artifact materialization failed", slog.Any("error", err))
			return cmdsupport.CommandError(commandMaterializeArtifact, err)
		}
		result.ModelPath = cacheCurrent
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

	resolvedDigest := strings.TrimSpace(artifactDigest)
	if resolvedDigest == "" {
		resolvedDigest = digestFromArtifactURI(artifactURI)
	}
	if resolvedDigest == "" {
		return "", "", errors.New("cache-root requires immutable artifact digest")
	}

	cacheRoot = filepath.Clean(cacheRoot)
	return filepath.Join(cacheRoot, cacheStoreDirName, resolvedDigest), filepath.Join(cacheRoot, cacheCurrentPath), nil
}

func digestFromArtifactURI(artifactURI string) string {
	artifactURI = strings.TrimSpace(artifactURI)
	if artifactURI == "" {
		return ""
	}
	before, after, ok := strings.Cut(artifactURI, "@")
	if !ok || strings.TrimSpace(before) == "" {
		return ""
	}
	return strings.TrimSpace(after)
}

func updateCurrentMaterializationLink(cacheRoot, targetPath string) error {
	cacheRoot = filepath.Clean(strings.TrimSpace(cacheRoot))
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if cacheRoot == "" || targetPath == "" {
		return errors.New("cache-root current symlink requires non-empty paths")
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return err
	}

	currentPath := filepath.Join(cacheRoot, cacheCurrentPath)
	relativeTarget, err := filepath.Rel(cacheRoot, targetPath)
	if err != nil {
		return err
	}
	tempLink := currentPath + ".tmp"
	_ = os.Remove(tempLink)
	if err := os.Symlink(relativeTarget, tempLink); err != nil {
		return err
	}
	if err := os.Rename(tempLink, currentPath); err != nil {
		_ = os.Remove(tempLink)
		return err
	}
	return nil
}
