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
	"net/http"
	"sort"

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
	if err := finalizeDescribedDirectUpload(ctx, helperClient, descriptor, checkpoint, state); err != nil {
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
) error {
	if err := checkpoint.markSealing(ctx, state.current); err != nil {
		return err
	}
	if err := helperClient.complete(ctx, state.session.SessionToken, descriptor.Digest, descriptor.Size, state.uploadedParts); err != nil {
		return err
	}
	return checkpoint.markLayerCompleted(ctx, descriptor)
}

func openDescribedDirectUploadSession(
	ctx context.Context,
	helperClient *directUploadClient,
	repository string,
	descriptor publishLayerDescriptor,
	checkpoint *directUploadCheckpoint,
) (
	directUploadSession,
	[]uploadedDirectPart,
	modelpackports.DirectUploadCurrentLayer,
	int64,
	int,
	error,
) {
	if current := checkpoint.currentLayer(descriptor); current != nil {
		if current.TotalSizeBytes != descriptor.Size {
			return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, 0, 0, fmt.Errorf("persisted direct upload size %d does not match descriptor size %d", current.TotalSizeBytes, descriptor.Size)
		}
		uploadedParts, err := helperClient.listParts(ctx, current.SessionToken)
		if err != nil {
			if !isDirectUploadStatus(err, http.StatusNotFound) {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, 0, 0, err
			}
		} else {
			offset, partNumber, err := nextDirectUploadPosition(uploadedParts)
			if err != nil {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, 0, 0, err
			}
			if offset < current.UploadedSizeBytes {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, 0, 0, fmt.Errorf("remote direct upload offset %d is behind persisted offset %d", offset, current.UploadedSizeBytes)
			}
			current.UploadedSizeBytes = offset
			if err := checkpoint.saveRunningLayer(ctx, *current, modelpackports.DirectUploadStateStageResumed); err != nil {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, 0, 0, err
			}
			return directUploadSession{
				SessionToken:  current.SessionToken,
				PartSizeBytes: current.PartSizeBytes,
			}, uploadedParts, *current, offset, partNumber, nil
		}
	}

	session, err := helperClient.start(ctx, repository)
	if err != nil {
		return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, 0, 0, err
	}
	current := modelpackports.DirectUploadCurrentLayer{
		Key:               directUploadLayerKey(descriptor),
		SessionToken:      session.SessionToken,
		PartSizeBytes:     session.PartSizeBytes,
		TotalSizeBytes:    descriptor.Size,
		UploadedSizeBytes: 0,
	}
	if err := checkpoint.saveRunningLayer(ctx, current, modelpackports.DirectUploadStateStageStarting); err != nil {
		return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, 0, 0, err
	}
	return session, nil, current, 0, 1, nil
}

func uploadDirectBlobPart(
	ctx context.Context,
	helperClient *directUploadClient,
	session directUploadSession,
	layer modelpackports.PublishLayer,
	offset int64,
	partNumber int,
	totalSize int64,
) (uploadedDirectPart, error) {
	chunkLength := nextDirectUploadChunkLength(offset, totalSize, session.PartSizeBytes)
	presignedURL, err := helperClient.presignPart(ctx, session.SessionToken, partNumber)
	if err != nil {
		return uploadedDirectPart{}, err
	}

	body, err := openPublishLayerRange(ctx, layer, offset, chunkLength)
	if err != nil {
		return uploadedDirectPart{}, err
	}
	defer body.Close()

	return helperClient.uploadPart(ctx, presignedURL, body, chunkLength, partNumber)
}

func recoverDirectBlobUpload(
	ctx context.Context,
	helperClient *directUploadClient,
	sessionToken string,
	cause error,
) ([]uploadedDirectPart, int64, int, error) {
	recoveredParts, err := helperClient.listParts(ctx, sessionToken)
	if err != nil {
		return nil, 0, 0, errors.Join(cause, err)
	}
	offset, partNumber, err := nextDirectUploadPosition(recoveredParts)
	if err != nil {
		return nil, 0, 0, errors.Join(cause, err)
	}
	return recoveredParts, offset, partNumber, nil
}

func nextDirectUploadChunkLength(offset, totalSize, partSizeBytes int64) int64 {
	if remaining := totalSize - offset; remaining < partSizeBytes {
		return remaining
	}
	return partSizeBytes
}

func normalizeUploadedDirectParts(parts []uploadedDirectPart) ([]uploadedDirectPart, error) {
	normalized := make([]uploadedDirectPart, 0, len(parts))
	for _, part := range parts {
		if part.PartNumber <= 0 {
			return nil, fmt.Errorf("uploaded direct part number must be positive, got %d", part.PartNumber)
		}
		if part.SizeBytes <= 0 {
			return nil, fmt.Errorf("uploaded direct part size must be positive, got %d", part.SizeBytes)
		}
		if part.ETag == "" {
			return nil, fmt.Errorf("uploaded direct part %d is missing ETag", part.PartNumber)
		}
		normalized = append(normalized, uploadedDirectPart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
			SizeBytes:  part.SizeBytes,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].PartNumber < normalized[j].PartNumber
	})
	for index := range normalized {
		expected := index + 1
		if normalized[index].PartNumber != expected {
			return nil, fmt.Errorf("uploaded direct parts are not contiguous: got part %d, want %d", normalized[index].PartNumber, expected)
		}
	}
	return normalized, nil
}

func nextDirectUploadPosition(parts []uploadedDirectPart) (int64, int, error) {
	normalized, err := normalizeUploadedDirectParts(parts)
	if err != nil {
		return 0, 0, err
	}
	offset := int64(0)
	for _, part := range normalized {
		offset += part.SizeBytes
	}
	return offset, len(normalized) + 1, nil
}
