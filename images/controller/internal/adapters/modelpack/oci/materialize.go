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

package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type Materializer struct{}

type materializedMarker struct {
	Version     string `json:"version"`
	ArtifactURI string `json:"artifactURI"`
	Digest      string `json:"digest"`
	MediaType   string `json:"mediaType,omitempty"`
	Family      string `json:"family,omitempty"`
	ModelPath   string `json:"modelPath"`
	ReadyAt     string `json:"readyAt"`
}

func NewMaterializer() *Materializer {
	return &Materializer{}
}

func (m *Materializer) Materialize(ctx context.Context, input modelpackports.MaterializeInput, auth modelpackports.RegistryAuth) (modelpackports.MaterializeResult, error) {
	if m == nil {
		return modelpackports.MaterializeResult{}, errors.New("OCI materializer must not be nil")
	}
	if err := validateMaterializeInput(input); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	logger := slog.Default().With(
		slog.String("artifactURI", strings.TrimSpace(input.ArtifactURI)),
		slog.String("destinationDir", strings.TrimSpace(input.DestinationDir)),
	)

	inspectStarted := time.Now()
	logger.Info("oci materialization remote inspect started")
	payload, err := InspectRemote(ctx, input.ArtifactURI, auth)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	if err := ValidatePayload(payload); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	logger.Info("oci materialization remote inspect completed", slog.Int64("durationMs", time.Since(inspectStarted).Milliseconds()))

	digest := ArtifactDigest(payload)
	if strings.TrimSpace(digest) == "" {
		return modelpackports.MaterializeResult{}, errors.New("remote artifact digest must not be empty")
	}
	if err := validateExpectedDigest(input, digest); err != nil {
		return modelpackports.MaterializeResult{}, err
	}

	result, reused, err := maybeReuseMaterialization(input.DestinationDir, digest)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	if reused {
		result.Digest = digest
		result.MediaType = ArtifactMediaType(payload)
		logger.Info(
			"oci materialization reused existing destination",
			slog.String("artifactDigest", digest),
			slog.String("modelPath", result.ModelPath),
			slog.String("markerPath", result.MarkerPath),
		)
		return result, nil
	}

	return materializeFresh(ctx, input, auth, payload)
}

func validateMaterializeInput(input modelpackports.MaterializeInput) error {
	if strings.TrimSpace(input.ArtifactURI) == "" {
		return errors.New("artifact URI must not be empty")
	}
	if strings.TrimSpace(input.DestinationDir) == "" {
		return errors.New("destination directory must not be empty")
	}
	return nil
}

func materializeFresh(
	ctx context.Context,
	input modelpackports.MaterializeInput,
	auth modelpackports.RegistryAuth,
	payload InspectPayload,
) (modelpackports.MaterializeResult, error) {
	digest := ArtifactDigest(payload)
	logger := slog.Default().With(
		slog.String("artifactURI", strings.TrimSpace(input.ArtifactURI)),
		slog.String("artifactDigest", strings.TrimSpace(digest)),
		slog.String("destinationDir", strings.TrimSpace(input.DestinationDir)),
	)

	client, err := RegistryHTTPClient(auth)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}

	stagingRoot, cleanupOnFailure, err := prepareMaterializationRoot(input, digest, payload)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	success := false
	defer func() {
		if !success && cleanupOnFailure {
			_ = os.RemoveAll(stagingRoot)
		}
	}()
	logger.Debug("oci materialization staging prepared", slog.String("stagingRoot", stagingRoot))

	extractStarted := time.Now()
	logger.Info("oci materialization layer extraction started", slog.Int("layerCount", layerCount(payload)))
	if err := extractLayers(ctx, client, input.ArtifactURI, auth, payload, stagingRoot); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	logger.Info("oci materialization layer extraction completed", slog.Int64("durationMs", time.Since(extractStarted).Milliseconds()))

	modelPath, err := resolveModelPath(stagingRoot, payload)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	modelPath, err = ensureMaterializedModelContract(stagingRoot, modelPath)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	if err := removeChunkMaterializeState(stagingRoot); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	modelRelativePath, err := filepath.Rel(stagingRoot, modelPath)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}

	if err := replaceMaterializedDestination(stagingRoot, input.DestinationDir); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	success = true
	logger.Info("oci materialization destination replaced", slog.String("destinationDir", input.DestinationDir))

	finalModelPath := input.DestinationDir
	if strings.TrimSpace(modelRelativePath) != "." && strings.TrimSpace(modelRelativePath) != "" {
		finalModelPath = filepath.Join(input.DestinationDir, modelRelativePath)
	}
	markerPath, err := writeMarker(input, digest, ArtifactMediaType(payload), finalModelPath)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	logger.Info("oci materialization marker written", slog.String("markerPath", markerPath), slog.String("modelPath", finalModelPath))

	return modelpackports.MaterializeResult{
		ModelPath:  finalModelPath,
		Digest:     digest,
		MediaType:  ArtifactMediaType(payload),
		MarkerPath: markerPath,
	}, nil
}

func prepareMaterializationRoot(
	input modelpackports.MaterializeInput,
	digest string,
	payload InspectPayload,
) (string, bool, error) {
	if payloadUsesChunkedLayout(payload) {
		root := chunkMaterializeWorkRoot(input.DestinationDir)
		if err := prepareChunkMaterializeWorkRoot(root, digest); err != nil {
			return "", false, err
		}
		return root, false, nil
	}
	stagingRoot, err := os.MkdirTemp(materializationParent(input.DestinationDir), ".ai-models-materialize-")
	if err != nil {
		return "", false, err
	}
	return stagingRoot, true, nil
}

func layerCount(payload InspectPayload) int {
	manifest, _ := payload["manifest"].(map[string]any)
	layers, _ := manifest["layers"].([]any)
	return len(layers)
}

func materializationParent(destination string) string {
	parent := filepath.Dir(destination)
	if strings.TrimSpace(parent) == "" || parent == "." {
		return "."
	}
	return parent
}

func validateExpectedDigest(input modelpackports.MaterializeInput, actual string) error {
	expected := strings.TrimSpace(input.ArtifactDigest)
	if expected == "" {
		expected = digestFromOCIReference(input.ArtifactURI)
	}
	if expected == "" {
		return nil
	}
	if expected != actual {
		return fmt.Errorf("published artifact digest mismatch: expected %q, got %q", expected, actual)
	}
	return nil
}

func writeMarker(input modelpackports.MaterializeInput, digest, mediaType, modelPath string) (string, error) {
	markerPath := nodecache.MarkerPath(input.DestinationDir)
	marker := materializedMarker{
		Version:     "v1",
		ArtifactURI: strings.TrimSpace(input.ArtifactURI),
		Digest:      strings.TrimSpace(digest),
		MediaType:   strings.TrimSpace(mediaType),
		Family:      strings.TrimSpace(input.ArtifactFamily),
		ModelPath:   modelPath,
		ReadyAt:     time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(markerPath, payload, 0o644); err != nil {
		return "", err
	}
	return markerPath, nil
}
