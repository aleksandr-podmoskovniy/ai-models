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
		uploadedPart, err := uploadDirectBlobPart(ctx, helperClient, state.session, layer, state.offset, state.partNumber, totalSize)
		if err == nil {
			state.uploadedParts = append(state.uploadedParts, uploadedPart)
			state.offset += uploadedPart.SizeBytes
			state.partNumber++
			recoveries = 0
		} else {
			nextParts, nextOffset, nextPartNumber, recoveryErr := recoverDirectBlobUpload(ctx, helperClient, state.session.SessionToken, err)
			if recoveryErr != nil {
				return recoveryErr
			}
			state.uploadedParts = nextParts
			state.offset = nextOffset
			state.partNumber = nextPartNumber
			recoveries++
			if recoveries > blobUploadRecoveryAttempts {
				return err
			}
		}
		state.current.UploadedSizeBytes = state.offset
		if err := checkpoint.saveRunningLayer(ctx, state.current, modelpackports.DirectUploadStateStageUploading); err != nil {
			return err
		}
	}
	return nil
}

func finalizeDescribedDirectUpload(
	ctx context.Context,
	helperClient *directUploadClient,
	descriptor publishLayerDescriptor,
	checkpoint *directUploadCheckpoint,
	state describedDirectUploadState,
	logger *slog.Logger,
) error {
	_, err := completeDirectUpload(
		ctx,
		helperClient,
		checkpoint,
		state.session,
		state.current,
		state.uploadedParts,
		descriptor.Digest,
		descriptor.Size,
		logger,
		func(result directUploadCompleteResult) (publishLayerDescriptor, error) {
			if err := verifyDirectUploadCompleteResult(result, descriptor.Digest, descriptor.Size); err != nil {
				return publishLayerDescriptor{}, err
			}
			return descriptor, nil
		},
		"layerTargetPath", descriptor.TargetPath,
		"layerDigest", descriptor.Digest,
		"layerSizeBytes", descriptor.Size,
	)
	return err
}

func completeDirectUpload(
	ctx context.Context,
	helperClient *directUploadClient,
	checkpoint *directUploadCheckpoint,
	session directUploadSession,
	current modelpackports.DirectUploadCurrentLayer,
	uploadedParts []uploadedDirectPart,
	expectedDigest string,
	expectedSize int64,
	logger *slog.Logger,
	buildDescriptor func(directUploadCompleteResult) (publishLayerDescriptor, error),
	startArgs ...any,
) (publishLayerDescriptor, error) {
	if err := checkpoint.markSealing(ctx, current); err != nil {
		return publishLayerDescriptor{}, err
	}

	completeStarted := time.Now()
	if logger != nil {
		logger.Info("native modelpack direct upload sealing started", append(startArgs, "uploadedPartCount", len(uploadedParts))...)
	}
	completeResult, err := helperClient.complete(ctx, session.SessionToken, expectedDigest, expectedSize, uploadedParts)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	descriptor, err := buildDescriptor(completeResult)
	if err != nil {
		return publishLayerDescriptor{}, err
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

func verifyDirectUploadCompleteResult(
	result directUploadCompleteResult,
	expectedDigest string,
	expectedSize int64,
) error {
	if result.SizeBytes != expectedSize {
		return fmt.Errorf("DMCR verified sizeBytes %d does not match expected sizeBytes %d", result.SizeBytes, expectedSize)
	}
	if result.Digest != expectedDigest {
		return fmt.Errorf("DMCR verified digest %q does not match expected digest %q", result.Digest, expectedDigest)
	}
	return nil
}
