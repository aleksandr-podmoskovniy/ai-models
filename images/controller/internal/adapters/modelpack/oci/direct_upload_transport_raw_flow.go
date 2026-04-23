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
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"log/slog"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type rawDirectUploadState struct {
	session       directUploadSession
	uploadedParts []uploadedDirectPart
	current       modelpackports.DirectUploadCurrentLayer
	hasher        hash.Hash
	totalSize     int64
	offset        int64
	partNumber    int
}

func prepareRawDirectUpload(
	ctx context.Context,
	input modelpackports.PublishInput,
	auth modelpackports.RegistryAuth,
	layer modelpackports.PublishLayer,
	plan publishLayerDescriptor,
	checkpoint *directUploadCheckpoint,
) (*directUploadClient, rawDirectUploadState, error) {
	parsedReference, err := parseOCIReference(input.ArtifactURI)
	if err != nil {
		return nil, rawDirectUploadState{}, err
	}
	helperClient, err := newDirectUploadClient(input, auth)
	if err != nil {
		return nil, rawDirectUploadState{}, err
	}

	totalSize, err := rawPublishLayerSize(layer)
	if err != nil {
		return nil, rawDirectUploadState{}, err
	}
	if totalSize <= 0 {
		return nil, rawDirectUploadState{}, errors.New("raw ModelPack layer size must be positive")
	}

	session, uploadedParts, current, hasher, offset, partNumber, err := openRawDirectUploadSession(
		ctx,
		helperClient,
		parsedReference.Repository,
		layer,
		plan,
		totalSize,
		checkpoint,
	)
	if err != nil {
		return nil, rawDirectUploadState{}, err
	}
	if session.PartSizeBytes <= 0 {
		return nil, rawDirectUploadState{}, errors.New("DMCR direct upload session returned non-positive part size")
	}

	return helperClient, rawDirectUploadState{
		session:       session,
		uploadedParts: uploadedParts,
		current:       current,
		hasher:        hasher,
		totalSize:     totalSize,
		offset:        offset,
		partNumber:    partNumber,
	}, nil
}

func uploadRawDirectLayerParts(
	ctx context.Context,
	helperClient *directUploadClient,
	layer modelpackports.PublishLayer,
	checkpoint *directUploadCheckpoint,
	state *rawDirectUploadState,
) error {
	for state.offset < state.totalSize {
		if err := uploadNextRawDirectLayerPart(ctx, helperClient, layer, checkpoint, state); err != nil {
			return err
		}
	}
	return nil
}

func uploadNextRawDirectLayerPart(
	ctx context.Context,
	helperClient *directUploadClient,
	layer modelpackports.PublishLayer,
	checkpoint *directUploadCheckpoint,
	state *rawDirectUploadState,
) error {
	chunkLength := nextDirectUploadChunkLength(state.offset, state.totalSize, state.session.PartSizeBytes)
	payload, err := readPublishLayerChunk(ctx, layer, state.offset, chunkLength)
	if err != nil {
		return err
	}

	state.uploadedParts, err = uploadOrRecoverRawDirectBlobPart(
		ctx,
		helperClient,
		state.session.SessionToken,
		payload,
		state.partNumber,
		state.uploadedParts,
	)
	if err != nil {
		return err
	}
	if _, err := state.hasher.Write(payload); err != nil {
		return err
	}

	state.offset += int64(len(payload))
	state.partNumber++
	state.current.UploadedSizeBytes = state.offset
	state.current.DigestState, err = marshalDirectUploadDigestState(state.hasher)
	if err != nil {
		return err
	}
	return checkpoint.saveRunningLayer(ctx, state.current, modelpackports.DirectUploadStateStageUploading)
}

func finalizeRawDirectUpload(
	ctx context.Context,
	helperClient *directUploadClient,
	plan publishLayerDescriptor,
	checkpoint *directUploadCheckpoint,
	state rawDirectUploadState,
	logger *slog.Logger,
) (publishLayerDescriptor, error) {
	expectedDigest := "sha256:" + hex.EncodeToString(state.hasher.Sum(nil))
	if err := checkpoint.markSealing(ctx, state.current); err != nil {
		return publishLayerDescriptor{}, err
	}
	completeStarted := time.Now()
	if logger != nil {
		logger.Info(
			"native modelpack direct upload sealing started",
			slog.String("layerTargetPath", plan.TargetPath),
			slog.String("expectedLayerDigest", expectedDigest),
			slog.Int64("expectedLayerSizeBytes", state.totalSize),
			slog.Int("uploadedPartCount", len(state.uploadedParts)),
		)
	}
	completeResult, err := helperClient.complete(ctx, state.session.SessionToken, expectedDigest, state.totalSize, state.uploadedParts)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	if completeResult.SizeBytes != state.totalSize {
		return publishLayerDescriptor{}, fmt.Errorf("DMCR verified sizeBytes %d does not match layer sizeBytes %d", completeResult.SizeBytes, state.totalSize)
	}
	if completeResult.Digest != expectedDigest {
		return publishLayerDescriptor{}, fmt.Errorf("DMCR verified digest %q does not match locally computed digest %q", completeResult.Digest, expectedDigest)
	}
	descriptor := publishLayerDescriptor{
		Digest:      completeResult.Digest,
		DiffID:      completeResult.Digest,
		Size:        completeResult.SizeBytes,
		MediaType:   plan.MediaType,
		TargetPath:  plan.TargetPath,
		Base:        plan.Base,
		Format:      plan.Format,
		Compression: modelpackports.LayerCompressionNone,
	}
	if logger != nil {
		logger.Info(
			"native modelpack direct upload sealing completed",
			slog.String("layerTargetPath", descriptor.TargetPath),
			slog.String("layerDigest", descriptor.Digest),
			slog.Int64("durationMs", time.Since(completeStarted).Milliseconds()),
		)
	}
	if err := checkpoint.markLayerCompleted(ctx, descriptor); err != nil {
		return publishLayerDescriptor{}, err
	}
	return descriptor, nil
}
