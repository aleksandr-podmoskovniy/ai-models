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
	"log/slog"
	"net/http"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type describedDirectUploadState struct {
	session       directUploadSession
	uploadedParts []uploadedDirectPart
	current       modelpackports.DirectUploadCurrentLayer
	offset        int64
	partNumber    int
}

func pushDescribedLayerDirectToBackingStorage(
	ctx context.Context,
	_ *http.Client,
	input modelpackports.PublishInput,
	auth modelpackports.RegistryAuth,
	layer modelpackports.PublishLayer,
	descriptor publishLayerDescriptor,
	checkpoint *directUploadCheckpoint,
	logger *slog.Logger,
) error {
	helperClient, state, err := prepareDescribedDirectUpload(ctx, input, auth, descriptor, checkpoint)
	if err != nil {
		return err
	}

	completeStarted := false
	completed := false
	defer func() {
		if completed || completeStarted {
			return
		}
		_ = helperClient.abort(ctx, state.session.SessionToken)
	}()

	if err := uploadDescribedLayerParts(ctx, helperClient, layer, descriptor.Size, checkpoint, &state); err != nil {
		return err
	}

	completeStarted = true
	if err := finalizeDescribedDirectUpload(ctx, helperClient, descriptor, checkpoint, state, logger); err != nil {
		return err
	}
	completed = true
	return nil
}

func prepareDescribedDirectUpload(
	ctx context.Context,
	input modelpackports.PublishInput,
	auth modelpackports.RegistryAuth,
	descriptor publishLayerDescriptor,
	checkpoint *directUploadCheckpoint,
) (*directUploadClient, describedDirectUploadState, error) {
	parsedReference, err := parseOCIReference(input.ArtifactURI)
	if err != nil {
		return nil, describedDirectUploadState{}, err
	}
	helperClient, err := newDirectUploadClient(input, auth)
	if err != nil {
		return nil, describedDirectUploadState{}, err
	}

	session, uploadedParts, current, offset, partNumber, err := openDescribedDirectUploadSession(
		ctx,
		helperClient,
		parsedReference.Repository,
		descriptor,
		checkpoint,
	)
	if err != nil {
		return nil, describedDirectUploadState{}, err
	}
	if session.PartSizeBytes <= 0 {
		return nil, describedDirectUploadState{}, errors.New("DMCR direct upload session returned non-positive part size")
	}

	return helperClient, describedDirectUploadState{
		session:       session,
		uploadedParts: uploadedParts,
		current:       current,
		offset:        offset,
		partNumber:    partNumber,
	}, nil
}

func uploadDescribedLayerParts(
	ctx context.Context,
	helperClient *directUploadClient,
	layer modelpackports.PublishLayer,
	totalSize int64,
	checkpoint *directUploadCheckpoint,
	state *describedDirectUploadState,
) error {
	recoveries := 0
	for state.offset < totalSize {
		recovered, err := uploadNextDescribedLayerPart(ctx, helperClient, layer, totalSize, checkpoint, state)
		if err == nil {
			recoveries = 0
			continue
		}
		if !recovered {
			return err
		}
		recoveries++
		if recoveries > blobUploadRecoveryAttempts {
			return err
		}
	}
	return nil
}

func uploadNextDescribedLayerPart(
	ctx context.Context,
	helperClient *directUploadClient,
	layer modelpackports.PublishLayer,
	totalSize int64,
	checkpoint *directUploadCheckpoint,
	state *describedDirectUploadState,
) (bool, error) {
	uploadedPart, err := uploadDirectBlobPart(ctx, helperClient, state.session, layer, state.offset, state.partNumber, totalSize)
	if err == nil {
		state.uploadedParts = append(state.uploadedParts, uploadedPart)
		state.offset += uploadedPart.SizeBytes
		state.partNumber++
		state.current.UploadedSizeBytes = state.offset
		return false, checkpoint.saveRunningLayer(ctx, state.current, modelpackports.DirectUploadStateStageUploading)
	}

	nextParts, nextOffset, nextPartNumber, recoveryErr := recoverDirectBlobUpload(ctx, helperClient, state.session.SessionToken, err)
	if recoveryErr != nil {
		return false, recoveryErr
	}
	state.uploadedParts = nextParts
	state.offset = nextOffset
	state.partNumber = nextPartNumber
	state.current.UploadedSizeBytes = state.offset
	if saveErr := checkpoint.saveRunningLayer(ctx, state.current, modelpackports.DirectUploadStateStageUploading); saveErr != nil {
		return false, saveErr
	}
	return true, err
}

func finalizeDescribedDirectUpload(
	ctx context.Context,
	helperClient *directUploadClient,
	descriptor publishLayerDescriptor,
	checkpoint *directUploadCheckpoint,
	state describedDirectUploadState,
	logger *slog.Logger,
) error {
	if err := checkpoint.markSealing(ctx, state.current); err != nil {
		return err
	}
	completeStarted := time.Now()
	if logger != nil {
		logger.Info(
			"native modelpack direct upload sealing started",
			slog.String("layerTargetPath", descriptor.TargetPath),
			slog.String("layerDigest", descriptor.Digest),
			slog.Int64("layerSizeBytes", descriptor.Size),
			slog.Int("uploadedPartCount", len(state.uploadedParts)),
		)
	}
	if err := helperClient.complete(ctx, state.session.SessionToken, descriptor.Digest, descriptor.Size, state.uploadedParts); err != nil {
		return err
	}
	if logger != nil {
		logger.Info(
			"native modelpack direct upload sealing completed",
			slog.String("layerTargetPath", descriptor.TargetPath),
			slog.String("layerDigest", descriptor.Digest),
			slog.Int64("durationMs", time.Since(completeStarted).Milliseconds()),
		)
	}
	return checkpoint.markLayerCompleted(ctx, descriptor)
}
