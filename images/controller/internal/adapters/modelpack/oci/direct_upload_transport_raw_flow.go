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
	"hash"
	"log/slog"

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
	totalSize, err := rawPublishLayerSize(layer)
	if err != nil {
		return nil, rawDirectUploadState{}, err
	}
	if totalSize <= 0 {
		return nil, rawDirectUploadState{}, errors.New("raw ModelPack layer size must be positive")
	}
	parsedReference, err := parseOCIReference(input.ArtifactURI)
	if err != nil {
		return nil, rawDirectUploadState{}, err
	}
	helperClient, err := newDirectUploadClient(input, auth)
	if err != nil {
		return nil, rawDirectUploadState{}, err
	}
	session, uploadedParts, current, nextHasher, offset, partNumber, err := openRawDirectUploadSession(
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
		hasher:        nextHasher,
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
		if err := checkpoint.saveRunningLayer(ctx, state.current, modelpackports.DirectUploadStateStageUploading); err != nil {
			return err
		}
	}
	return nil
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
	return completeDirectUpload(
		ctx,
		helperClient,
		checkpoint,
		state.session,
		state.current,
		state.uploadedParts,
		expectedDigest,
		state.totalSize,
		logger,
		func(result directUploadCompleteResult) (publishLayerDescriptor, error) {
			if err := verifyDirectUploadCompleteResult(result, expectedDigest, state.totalSize); err != nil {
				return publishLayerDescriptor{}, err
			}
			return publishLayerDescriptor{
				Digest:      result.Digest,
				DiffID:      result.Digest,
				Size:        result.SizeBytes,
				MediaType:   plan.MediaType,
				TargetPath:  plan.TargetPath,
				Base:        plan.Base,
				Format:      plan.Format,
				Compression: modelpackports.LayerCompressionNone,
			}, nil
		},
		"layerTargetPath", plan.TargetPath,
		"expectedLayerDigest", expectedDigest,
		"expectedLayerSizeBytes", state.totalSize,
	)
}
