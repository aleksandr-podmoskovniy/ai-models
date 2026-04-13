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
	"os"
	"path/filepath"
	"strings"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const markerFileName = ".ai-models-materialized.json"

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

	payload, err := InspectRemote(ctx, input.ArtifactURI, auth)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	if err := ValidatePayload(payload); err != nil {
		return modelpackports.MaterializeResult{}, err
	}

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

	client, err := RegistryHTTPClient(auth)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}

	stagingRoot, err := os.MkdirTemp(materializationParent(input.DestinationDir), ".ai-models-materialize-")
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(stagingRoot)
		}
	}()

	if err := extractLayers(ctx, client, input.ArtifactURI, auth, payload, stagingRoot); err != nil {
		return modelpackports.MaterializeResult{}, err
	}

	modelPath, err := resolveModelPath(stagingRoot, payload)
	if err != nil {
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

	finalModelPath := input.DestinationDir
	if strings.TrimSpace(modelRelativePath) != "." && strings.TrimSpace(modelRelativePath) != "" {
		finalModelPath = filepath.Join(input.DestinationDir, modelRelativePath)
	}
	markerPath, err := writeMarker(input, digest, ArtifactMediaType(payload), finalModelPath)
	if err != nil {
		return modelpackports.MaterializeResult{}, err
	}

	return modelpackports.MaterializeResult{
		ModelPath:  finalModelPath,
		Digest:     digest,
		MediaType:  ArtifactMediaType(payload),
		MarkerPath: markerPath,
	}, nil
}

func replaceMaterializedDestination(stagingRoot, destination string) error {
	parent := materializationParent(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}

	backupDir, hadExisting, err := stageExistingDestination(destination)
	if err != nil {
		return err
	}

	if err := os.Rename(stagingRoot, destination); err != nil {
		if hadExisting {
			_ = os.Rename(backupDir, destination)
		}
		return err
	}
	if hadExisting {
		return os.RemoveAll(backupDir)
	}
	return nil
}

func stageExistingDestination(destination string) (string, bool, error) {
	if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	backupDir := destination + ".previous"
	if err := os.RemoveAll(backupDir); err != nil {
		return "", false, err
	}
	if err := os.Rename(destination, backupDir); err != nil {
		return "", false, err
	}
	return backupDir, true, nil
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

func maybeReuseMaterialization(destination, digest string) (modelpackports.MaterializeResult, bool, error) {
	marker, err := readMarker(destination)
	if err != nil {
		return modelpackports.MaterializeResult{}, false, err
	}
	if marker == nil {
		return modelpackports.MaterializeResult{}, false, nil
	}
	if strings.TrimSpace(marker.Digest) != strings.TrimSpace(digest) {
		return modelpackports.MaterializeResult{}, false, nil
	}
	if strings.TrimSpace(marker.ModelPath) == "" {
		return modelpackports.MaterializeResult{}, false, nil
	}
	if _, err := os.Stat(marker.ModelPath); err != nil {
		return modelpackports.MaterializeResult{}, false, nil
	}
	return modelpackports.MaterializeResult{
		ModelPath:  marker.ModelPath,
		MarkerPath: filepath.Join(destination, markerFileName),
	}, true, nil
}

func readMarker(destination string) (*materializedMarker, error) {
	markerPath := filepath.Join(destination, markerFileName)
	body, err := os.ReadFile(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var marker materializedMarker
	if err := json.Unmarshal(body, &marker); err != nil {
		return nil, fmt.Errorf("failed to decode materialization marker: %w", err)
	}
	return &marker, nil
}

func writeMarker(input modelpackports.MaterializeInput, digest, mediaType, modelPath string) (string, error) {
	markerPath := filepath.Join(input.DestinationDir, markerFileName)
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
