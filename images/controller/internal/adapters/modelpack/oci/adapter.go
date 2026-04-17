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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
	if strings.TrimSpace(input.ModelDir) == "" {
		return modelpackports.PublishResult{}, errors.New("model directory must not be empty")
	}
	if strings.TrimSpace(input.ArtifactURI) == "" {
		return modelpackports.PublishResult{}, errors.New("artifact URI must not be empty")
	}

	logger := slog.Default().With(
		slog.String("artifactURI", strings.TrimSpace(input.ArtifactURI)),
		slog.String("modelDir", strings.TrimSpace(input.ModelDir)),
		slog.String("publisher", "ai-models-native-oci"),
	)
	client, err := RegistryHTTPClient(auth)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}

	layerStarted := time.Now()
	logger.Info("native modelpack layer stream started")
	layerDescriptor, err := streamModelLayer(ctx, client, input.ArtifactURI, auth, input.ModelDir)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	logger.Info(
		"native modelpack layer stream completed",
		slog.Int64("durationMs", time.Since(layerStarted).Milliseconds()),
		slog.String("layerDigest", layerDescriptor.Digest),
		slog.Int64("layerSizeBytes", layerDescriptor.Size),
	)

	configBytes, err := buildConfigBlob(layerDescriptor.DiffID)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	configDescriptor, err := newBlobDescriptor(configBytes)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	configStarted := time.Now()
	logger.Info("native modelpack config upload started")
	if err := uploadBlobFromReader(ctx, client, input.ArtifactURI, auth, bytes.NewReader(configBytes), int64(len(configBytes)), configDescriptor.Digest); err != nil {
		return modelpackports.PublishResult{}, err
	}
	logger.Info("native modelpack config upload completed", slog.Int64("durationMs", time.Since(configStarted).Milliseconds()), slog.String("configDigest", configDescriptor.Digest))

	manifestBytes, err := buildManifestBlob(configDescriptor, layerDescriptor)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	manifestStarted := time.Now()
	logger.Info("native modelpack manifest publish started")
	if err := putManifest(ctx, client, input.ArtifactURI, auth, manifestBytes); err != nil {
		return modelpackports.PublishResult{}, err
	}
	logger.Info("native modelpack manifest publish completed", slog.Int64("durationMs", time.Since(manifestStarted).Milliseconds()))

	inspectStarted := time.Now()
	logger.Info("modelpack remote inspect started")
	inspectPayload, err := InspectRemote(ctx, input.ArtifactURI, auth)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	if err := ValidatePayload(inspectPayload); err != nil {
		return modelpackports.PublishResult{}, err
	}

	digest := ArtifactDigest(inspectPayload)
	if strings.TrimSpace(digest) == "" {
		return modelpackports.PublishResult{}, errors.New("native modelpack inspect payload is missing digest")
	}
	logger.Info(
		"modelpack remote inspect completed",
		slog.Int64("durationMs", time.Since(inspectStarted).Milliseconds()),
		slog.String("artifactDigest", digest),
		slog.String("artifactMediaType", ArtifactMediaType(inspectPayload)),
		slog.Int64("artifactSizeBytes", InspectSizeBytes(inspectPayload)),
	)

	return modelpackports.PublishResult{
		Reference: immutableOCIReference(input.ArtifactURI, digest),
		Digest:    digest,
		MediaType: ArtifactMediaType(inspectPayload),
		SizeBytes: InspectSizeBytes(inspectPayload),
	}, nil
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

func uploadBlobFromReader(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	body io.Reader,
	size int64,
	digest string,
) error {
	uploadURL, err := initiateBlobUpload(ctx, client, reference, auth)
	if err != nil {
		return err
	}
	parsedUploadURL, err := url.Parse(uploadURL)
	if err != nil {
		return err
	}
	query := parsedUploadURL.Query()
	query.Set("digest", strings.TrimSpace(digest))
	parsedUploadURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, parsedUploadURL.String(), body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload modelpack blob: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("failed to upload modelpack blob: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

func uploadBlobChunk(
	ctx context.Context,
	client *http.Client,
	uploadURL string,
	auth modelpackports.RegistryAuth,
	body io.Reader,
) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, uploadURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to stream modelpack blob chunk: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("failed to stream modelpack blob chunk: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if location := strings.TrimSpace(resp.Header.Get("Location")); location != "" {
		return resolveUploadLocation(uploadURL, location)
	}

	return uploadURL, nil
}

func finalizeBlobUpload(
	ctx context.Context,
	client *http.Client,
	uploadURL string,
	auth modelpackports.RegistryAuth,
	digest string,
) error {
	parsedUploadURL, err := url.Parse(uploadURL)
	if err != nil {
		return err
	}
	query := parsedUploadURL.Query()
	query.Set("digest", strings.TrimSpace(digest))
	parsedUploadURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, parsedUploadURL.String(), nil)
	if err != nil {
		return err
	}
	req.ContentLength = 0
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to finalize modelpack blob upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("failed to finalize modelpack blob upload: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

func initiateBlobUpload(ctx context.Context, client *http.Client, reference string, auth modelpackports.RegistryAuth) (string, error) {
	parsed, err := parseOCIReference(reference)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.uploadURL(), nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to initiate modelpack blob upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("failed to initiate modelpack blob upload: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return resolveUploadLocation(parsed.uploadURL(), resp.Header.Get("Location"))
}

func putManifest(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	manifest []byte,
) error {
	manifestURL, err := RegistryManifestURL(reference)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, manifestURL, bytes.NewReader(manifest))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ManifestMediaType)
	req.ContentLength = int64(len(manifest))
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to publish modelpack manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("failed to publish modelpack manifest: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
