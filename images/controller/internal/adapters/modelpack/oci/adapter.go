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

const (
	deleteVerificationTimeout  = 5 * time.Second
	deleteVerificationInterval = 200 * time.Millisecond
)

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
	if strings.TrimSpace(input.DirectUploadEndpoint) == "" {
		return modelpackports.PublishResult{}, errors.New("direct upload endpoint must not be empty")
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

	layerPlanStarted := time.Now()
	logger.Info("native modelpack layer plan validation started")
	layerPlans, err := planPublishLayers(layers)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	logger.Info(
		"native modelpack layer plan validation completed",
		slog.Int64("durationMs", time.Since(layerPlanStarted).Milliseconds()),
		slog.Int("layerCount", len(layerPlans)),
	)
	layerDescriptors, err := uploadPublishLayers(ctx, client, input, auth, layers, layerPlans, logger)
	if err != nil {
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
	req.Header.Set("Accept", ManifestAcceptHeader)
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete remote ModelPack manifest: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusNotFound:
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return waitForRemoteManifestDeletion(ctx, client, parsed, digest, auth)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("failed to delete remote ModelPack manifest: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func waitForRemoteManifestDeletion(
	ctx context.Context,
	client *http.Client,
	reference registryReference,
	digest string,
	auth modelpackports.RegistryAuth,
) error {
	verifyCtx := ctx
	cancel := func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		verifyCtx, cancel = context.WithTimeout(ctx, deleteVerificationTimeout)
	}
	defer cancel()

	ticker := time.NewTicker(deleteVerificationInterval)
	defer ticker.Stop()
	manifestObserved := false

	for {
		exists, err := remoteManifestExists(verifyCtx, client, reference, digest, auth)
		if err != nil {
			if manifestObserved && errors.Is(verifyCtx.Err(), context.DeadlineExceeded) {
				return remoteManifestStillVisibleError(digest)
			}
			return err
		}
		if !exists {
			return nil
		}
		manifestObserved = true

		select {
		case <-verifyCtx.Done():
			return remoteManifestStillVisibleError(digest)
		case <-ticker.C:
		}
	}
}

func remoteManifestStillVisibleError(digest string) error {
	return fmt.Errorf("remote ModelPack manifest %q still exists after delete acknowledgement", digest)
}

func remoteManifestExists(
	ctx context.Context,
	client *http.Client,
	reference registryReference,
	digest string,
	auth modelpackports.RegistryAuth,
) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reference.manifestURL(digest), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", ManifestAcceptHeader)
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to verify remote ModelPack deletion: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return false, nil
	case http.StatusOK:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return true, nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return false, fmt.Errorf(
			"failed to verify remote ModelPack deletion: status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}
}
