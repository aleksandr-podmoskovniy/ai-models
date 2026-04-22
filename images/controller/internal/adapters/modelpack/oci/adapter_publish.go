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
	"log/slog"
	"net/http"
	"strings"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func uploadPublishLayers(
	ctx context.Context,
	client *http.Client,
	input modelpackports.PublishInput,
	auth modelpackports.RegistryAuth,
	layers []modelpackports.PublishLayer,
	plans []publishLayerDescriptor,
	logger *slog.Logger,
) ([]publishLayerDescriptor, error) {
	checkpoint, err := loadDirectUploadCheckpoint(ctx, input)
	if err != nil {
		return nil, err
	}
	preparedLayers, totalSizeBytes, err := preparePublishLayerUploads(ctx, layers, plans)
	if err != nil {
		return nil, err
	}
	if err := checkpoint.ensureProgressPlan(ctx, len(preparedLayers), totalSizeBytes); err != nil {
		return nil, err
	}

	descriptors := make([]publishLayerDescriptor, 0, len(layers))
	for index, prepared := range preparedLayers {
		layerUploadStarted := time.Now()
		logger.Info(
			"native modelpack layer upload started",
			slog.Int("layerIndex", index),
			slog.String("layerTargetPath", prepared.Plan.TargetPath),
			slog.String("layerMediaType", prepared.Plan.MediaType),
		)
		descriptor, err := uploadPublishLayer(ctx, client, input, auth, prepared, checkpoint, logger)
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, descriptor)
		logger.Info(
			"native modelpack layer upload completed",
			slog.Int64("durationMs", time.Since(layerUploadStarted).Milliseconds()),
			slog.Int("layerIndex", index),
			slog.String("layerDigest", descriptor.Digest),
			slog.Int64("layerSizeBytes", descriptor.Size),
		)
	}
	return descriptors, nil
}

func uploadPublishLayer(
	ctx context.Context,
	client *http.Client,
	input modelpackports.PublishInput,
	auth modelpackports.RegistryAuth,
	prepared preparedPublishLayer,
	checkpoint *directUploadCheckpoint,
	logger *slog.Logger,
) (publishLayerDescriptor, error) {
	plan := prepared.Plan
	if checkpoint != nil {
		if completed, found, err := checkpoint.completedLayer(plan); err != nil {
			return publishLayerDescriptor{}, err
		} else if found {
			return completed, nil
		}
	}

	if prepared.Descriptor != nil {
		descriptor := *prepared.Descriptor
		if err := pushDescribedLayerDirectToBackingStorage(ctx, client, input, auth, prepared.Layer, descriptor, checkpoint, logger); err != nil {
			return publishLayerDescriptor{}, err
		}
		return descriptor, nil
	}

	return pushRawLayerDirectToBackingStorage(ctx, client, input, auth, prepared.Layer, plan, checkpoint, logger)
}

func uploadPublishConfig(
	ctx context.Context,
	client *http.Client,
	artifactURI string,
	auth modelpackports.RegistryAuth,
	descriptors []publishLayerDescriptor,
	logger *slog.Logger,
) (blobDescriptor, error) {
	configBytes, err := buildConfigBlob(publishedModelPath(descriptors), descriptors)
	if err != nil {
		return blobDescriptor{}, err
	}
	configDescriptor, err := newBlobDescriptor(configBytes)
	if err != nil {
		return blobDescriptor{}, err
	}
	configStarted := time.Now()
	logger.Info("native modelpack config upload started")
	if err := uploadBlobFromReader(ctx, client, artifactURI, auth, bytes.NewReader(configBytes), int64(len(configBytes)), configDescriptor.Digest); err != nil {
		return blobDescriptor{}, err
	}
	logger.Info(
		"native modelpack config upload completed",
		slog.Int64("durationMs", time.Since(configStarted).Milliseconds()),
		slog.String("configDigest", configDescriptor.Digest),
	)
	return configDescriptor, nil
}

func publishManifest(
	ctx context.Context,
	client *http.Client,
	artifactURI string,
	auth modelpackports.RegistryAuth,
	configDescriptor blobDescriptor,
	layerDescriptors []publishLayerDescriptor,
	logger *slog.Logger,
) error {
	manifestBytes, err := buildManifestBlob(configDescriptor, layerDescriptors)
	if err != nil {
		return err
	}
	manifestStarted := time.Now()
	logger.Info("native modelpack manifest publish started")
	if err := putManifest(ctx, client, artifactURI, auth, manifestBytes); err != nil {
		return err
	}
	logger.Info("native modelpack manifest publish completed", slog.Int64("durationMs", time.Since(manifestStarted).Milliseconds()))
	return nil
}

func inspectPublishedModelPack(
	ctx context.Context,
	artifactURI string,
	auth modelpackports.RegistryAuth,
	logger *slog.Logger,
) (modelpackports.PublishResult, error) {
	inspectStarted := time.Now()
	logger.Info("modelpack remote inspect started")
	inspectPayload, err := InspectRemote(ctx, artifactURI, auth)
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
		Reference: immutableOCIReference(artifactURI, digest),
		Digest:    digest,
		MediaType: ArtifactMediaType(inspectPayload),
		SizeBytes: InspectSizeBytes(inspectPayload),
	}, nil
}
