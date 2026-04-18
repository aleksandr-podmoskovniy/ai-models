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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const ManifestMediaType = "application/vnd.oci.image.manifest.v1+json"

type Adapter struct{}

type blobDescriptor struct {
	Digest string
	DiffID string
	Size   int64
}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Publish(ctx context.Context, input modelpackports.PublishInput, auth modelpackports.RegistryAuth) (modelpackports.PublishResult, error) {
	if a == nil {
		return modelpackports.PublishResult{}, errors.New("OCI modelpack adapter must not be nil")
	}
	if strings.TrimSpace(input.ArtifactURI) == "" {
		return modelpackports.PublishResult{}, errors.New("artifact URI must not be empty")
	}
	layers, err := defaultPublishLayers(input)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}

	logger := slog.Default().With(
		slog.String("artifactURI", strings.TrimSpace(input.ArtifactURI)),
		slog.Int("layerCount", len(layers)),
		slog.String("publisher", "ai-models-native-oci"),
	)
	client, err := RegistryHTTPClient(auth)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}

	layerDescribeStarted := time.Now()
	logger.Info("native modelpack layer descriptor precompute started")
	layerDescriptors, err := describePublishLayers(ctx, layers)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	logger.Info(
		"native modelpack layer descriptor precompute completed",
		slog.Int64("durationMs", time.Since(layerDescribeStarted).Milliseconds()),
		slog.Int("layerCount", len(layerDescriptors)),
	)
	if err := uploadPublishLayers(ctx, client, input.ArtifactURI, auth, layers, layerDescriptors, logger); err != nil {
		return modelpackports.PublishResult{}, err
	}

	configDescriptor, err := uploadPublishConfig(ctx, client, input.ArtifactURI, auth, layerDescriptors, logger)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}

	if err := publishManifest(ctx, client, input.ArtifactURI, auth, configDescriptor, layerDescriptors, logger); err != nil {
		return modelpackports.PublishResult{}, err
	}
	return inspectPublishedModelPack(ctx, input.ArtifactURI, auth, logger)
}

func (a *Adapter) Remove(ctx context.Context, reference string, auth modelpackports.RegistryAuth) error {
	if a == nil {
		return errors.New("OCI modelpack adapter must not be nil")
	}
	if strings.TrimSpace(reference) == "" {
		return errors.New("cleanup reference must not be empty")
	}

	client, err := RegistryHTTPClient(auth)
	if err != nil {
		return err
	}

	digest := digestFromOCIReference(reference)
	if digest == "" {
		payload, inspectErr := InspectRemote(ctx, reference, auth)
		if inspectErr != nil {
			return inspectErr
		}
		digest = ArtifactDigest(payload)
	}
	if strings.TrimSpace(digest) == "" {
		return fmt.Errorf("failed to resolve manifest digest for %q", reference)
	}

	parsed, err := parseOCIReference(reference)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, parsed.manifestURL(digest), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete remote ModelPack manifest: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusNotFound:
		return nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("failed to delete remote ModelPack manifest: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}
