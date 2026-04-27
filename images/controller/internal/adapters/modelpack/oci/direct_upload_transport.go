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

type directUploadOpenSessionHooks struct {
	prepareNew    func(*modelpackports.DirectUploadCurrentLayer) error
	prepareResume func(context.Context, *modelpackports.DirectUploadCurrentLayer, int64) error
}

type directUploadOpenSessionResult struct {
	session       directUploadSession
	uploadedParts []uploadedDirectPart
	current       modelpackports.DirectUploadCurrentLayer
	offset        int64
	partNumber    int
}

func openDirectUploadSession(
	ctx context.Context,
	helperClient *directUploadClient,
	repository string,
	plan publishLayerDescriptor,
	totalSize int64,
	checkpoint *directUploadCheckpoint,
	hooks directUploadOpenSessionHooks,
) (directUploadOpenSessionResult, error) {
	if current := checkpoint.currentLayer(plan); current != nil {
		if current.TotalSizeBytes != totalSize {
			return directUploadOpenSessionResult{}, fmt.Errorf("persisted direct upload size %d does not match layer size %d", current.TotalSizeBytes, totalSize)
		}
		uploadedParts, err := helperClient.listParts(ctx, current.SessionToken)
		switch {
		case err == nil:
			offset, partNumber, err := nextDirectUploadPosition(uploadedParts)
			if err != nil {
				return directUploadOpenSessionResult{}, err
			}
			if offset < current.UploadedSizeBytes {
				return directUploadOpenSessionResult{}, fmt.Errorf("remote direct upload offset %d is behind persisted offset %d", offset, current.UploadedSizeBytes)
			}
			if hooks.prepareResume != nil {
				if err := hooks.prepareResume(ctx, current, offset); err != nil {
					return directUploadOpenSessionResult{}, err
				}
			}
			current.UploadedSizeBytes = offset
			if err := checkpoint.saveRunningLayer(ctx, *current, modelpackports.DirectUploadStateStageResumed); err != nil {
				return directUploadOpenSessionResult{}, err
			}
			return directUploadOpenSessionResult{
				session:       directUploadSession{SessionToken: current.SessionToken, PartSizeBytes: current.PartSizeBytes},
				uploadedParts: uploadedParts,
				current:       *current,
				offset:        offset,
				partNumber:    partNumber,
			}, nil
		case !isDirectUploadStatus(err, http.StatusNotFound):
			return directUploadOpenSessionResult{}, err
		}
	}

	session, err := helperClient.start(ctx, repository)
	if err != nil {
		return directUploadOpenSessionResult{}, err
	}
	current := modelpackports.DirectUploadCurrentLayer{
		Key:               directUploadLayerKey(plan),
		SessionToken:      session.SessionToken,
		PartSizeBytes:     session.PartSizeBytes,
		TotalSizeBytes:    totalSize,
		UploadedSizeBytes: 0,
	}
	if hooks.prepareNew != nil {
		if err := hooks.prepareNew(&current); err != nil {
			return directUploadOpenSessionResult{}, err
		}
	}
	if err := checkpoint.saveRunningLayer(ctx, current, modelpackports.DirectUploadStateStageStarting); err != nil {
		return directUploadOpenSessionResult{}, err
	}
	return directUploadOpenSessionResult{session: session, current: current, offset: 0, partNumber: 1}, nil
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

	sessionResult, err := openDirectUploadSession(
		ctx,
		helperClient,
		parsedReference.Repository,
		descriptor,
		descriptor.Size,
		checkpoint,
		directUploadOpenSessionHooks{},
	)
	if err != nil {
		return nil, describedDirectUploadState{}, err
	}
	if sessionResult.session.PartSizeBytes <= 0 {
		return nil, describedDirectUploadState{}, errors.New("DMCR direct upload session returned non-positive part size")
	}

	return helperClient, describedDirectUploadState{
		session:       sessionResult.session,
		uploadedParts: sessionResult.uploadedParts,
		current:       sessionResult.current,
		offset:        sessionResult.offset,
		partNumber:    sessionResult.partNumber,
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
	retryWait := directUploadAPIInitialRetryWait
	for state.offset < totalSize {
		uploadedPart, err := uploadDirectBlobPart(ctx, helperClient, state.session, layer, state.offset, state.partNumber, totalSize)
		if err == nil {
			state.uploadedParts = append(state.uploadedParts, uploadedPart)
			state.offset += uploadedPart.SizeBytes
			state.partNumber++
			recoveries = 0
			retryWait = directUploadAPIInitialRetryWait
		} else {
			previousOffset := state.offset
			nextParts, nextOffset, nextPartNumber, recoveryErr := recoverDirectBlobUpload(ctx, helperClient, state.session.SessionToken, err)
			if recoveryErr != nil {
				return recoveryErr
			}
			state.uploadedParts = nextParts
			state.offset = nextOffset
			state.partNumber = nextPartNumber
			if state.offset > previousOffset {
				recoveries = 0
				retryWait = directUploadAPIInitialRetryWait
			} else if retryErr := waitDirectUploadRecoveryRetry(ctx, &recoveries, &retryWait, err); retryErr != nil {
				return retryErr
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
