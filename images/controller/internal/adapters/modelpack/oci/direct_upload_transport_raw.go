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
	"crypto/sha256"
	"encoding"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func pushRawLayerDirectToBackingStorage(
	ctx context.Context,
	_ *http.Client,
	input modelpackports.PublishInput,
	auth modelpackports.RegistryAuth,
	layer modelpackports.PublishLayer,
	plan publishLayerDescriptor,
	checkpoint *directUploadCheckpoint,
	logger *slog.Logger,
) (publishLayerDescriptor, error) {
	helperClient, state, err := prepareRawDirectUpload(ctx, input, auth, layer, plan, checkpoint)
	if err != nil {
		return publishLayerDescriptor{}, err
	}

	completeStarted := false
	completed := false
	defer func() {
		if completed || completeStarted {
			return
		}
		_ = helperClient.abort(ctx, state.session.SessionToken)
	}()

	if err := uploadRawDirectLayerParts(ctx, helperClient, layer, checkpoint, &state); err != nil {
		return publishLayerDescriptor{}, err
	}

	completeStarted = true
	descriptor, err := finalizeRawDirectUpload(ctx, helperClient, plan, checkpoint, state, logger)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	completed = true
	return descriptor, nil
}

func canOnePassUploadRawLayer(layer modelpackports.PublishLayer) bool {
	if layer.Format != modelpackports.LayerFormatRaw {
		return false
	}
	if layer.ObjectSource == nil {
		return true
	}
	_, ok := layer.ObjectSource.Reader.(modelpackports.PublishObjectRangeReader)
	return ok
}

func openRawDirectUploadSession(
	ctx context.Context,
	helperClient *directUploadClient,
	repository string,
	layer modelpackports.PublishLayer,
	plan publishLayerDescriptor,
	totalSize int64,
	checkpoint *directUploadCheckpoint,
) (
	directUploadSession,
	[]uploadedDirectPart,
	modelpackports.DirectUploadCurrentLayer,
	hash.Hash,
	int64,
	int,
	error,
) {
	if current := checkpoint.currentLayer(plan); current != nil {
		if current.TotalSizeBytes != totalSize {
			return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, fmt.Errorf("persisted direct upload size %d does not match raw layer size %d", current.TotalSizeBytes, totalSize)
		}
		hasher, err := restoreDirectUploadDigestState(current.DigestState)
		if err != nil {
			return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
		}
		uploadedParts, err := helperClient.listParts(ctx, current.SessionToken)
		if err != nil {
			if !isDirectUploadStatus(err, http.StatusNotFound) {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
			}
		} else {
			offset, partNumber, err := nextDirectUploadPosition(uploadedParts)
			if err != nil {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
			}
			if offset < current.UploadedSizeBytes {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, fmt.Errorf("remote direct upload offset %d is behind persisted offset %d", offset, current.UploadedSizeBytes)
			}
			if offset > current.UploadedSizeBytes {
				if err := hashPublishLayerRange(ctx, hasher, layer, current.UploadedSizeBytes, offset-current.UploadedSizeBytes); err != nil {
					return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
				}
			}
			current.UploadedSizeBytes = offset
			current.DigestState, err = marshalDirectUploadDigestState(hasher)
			if err != nil {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
			}
			if err := checkpoint.saveRunningLayer(ctx, *current, modelpackports.DirectUploadStateStageResumed); err != nil {
				return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
			}
			return directUploadSession{
				SessionToken:  current.SessionToken,
				PartSizeBytes: current.PartSizeBytes,
			}, uploadedParts, *current, hasher, offset, partNumber, nil
		}
	}

	session, err := helperClient.start(ctx, repository)
	if err != nil {
		return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
	}
	hasher := sha256.New()
	digestState, err := marshalDirectUploadDigestState(hasher)
	if err != nil {
		return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
	}
	current := modelpackports.DirectUploadCurrentLayer{
		Key:               directUploadLayerKey(plan),
		SessionToken:      session.SessionToken,
		PartSizeBytes:     session.PartSizeBytes,
		TotalSizeBytes:    totalSize,
		UploadedSizeBytes: 0,
		DigestState:       digestState,
	}
	if err := checkpoint.saveRunningLayer(ctx, current, modelpackports.DirectUploadStateStageStarting); err != nil {
		return directUploadSession{}, nil, modelpackports.DirectUploadCurrentLayer{}, nil, 0, 0, err
	}
	return session, nil, current, hasher, 0, 1, nil
}

func uploadOrRecoverRawDirectBlobPart(
	ctx context.Context,
	helperClient *directUploadClient,
	sessionToken string,
	payload []byte,
	partNumber int,
	uploadedParts []uploadedDirectPart,
) ([]uploadedDirectPart, error) {
	recoveries := 0
	for {
		uploadedPart, err := uploadBufferedDirectBlobPart(ctx, helperClient, sessionToken, payload, partNumber)
		if err == nil {
			return append(uploadedParts, uploadedPart), nil
		}

		nextUploadedParts, retry, recoveryErr := recoverRawDirectBlobPart(
			ctx,
			helperClient,
			sessionToken,
			payload,
			partNumber,
			uploadedParts,
			err,
		)
		if recoveryErr != nil {
			return nil, recoveryErr
		}
		if retry {
			recoveries++
			if recoveries > blobUploadRecoveryAttempts {
				return nil, err
			}
			continue
		}
		return nextUploadedParts, nil
	}
}

func recoverRawDirectBlobPart(
	ctx context.Context,
	helperClient *directUploadClient,
	sessionToken string,
	payload []byte,
	partNumber int,
	uploadedParts []uploadedDirectPart,
	cause error,
) ([]uploadedDirectPart, bool, error) {
	recoveredParts, err := helperClient.listParts(ctx, sessionToken)
	if err != nil {
		return nil, false, errors.Join(cause, err)
	}
	switch {
	case len(recoveredParts) == len(uploadedParts):
		return nil, true, nil
	case len(recoveredParts) == len(uploadedParts)+1:
		recoveredPart := recoveredParts[len(uploadedParts)]
		if recoveredPart.PartNumber != partNumber {
			return nil, false, errors.Join(cause, fmt.Errorf("direct upload recovery committed unexpected part %d, want %d", recoveredPart.PartNumber, partNumber))
		}
		if recoveredPart.SizeBytes != int64(len(payload)) {
			return nil, false, errors.Join(cause, fmt.Errorf("direct upload recovery returned part %d size %d, want %d", recoveredPart.PartNumber, recoveredPart.SizeBytes, len(payload)))
		}
		return recoveredParts, false, nil
	default:
		return nil, false, errors.Join(cause, fmt.Errorf("direct upload recovery returned %d parts, want %d or %d", len(recoveredParts), len(uploadedParts), len(uploadedParts)+1))
	}
}

func uploadBufferedDirectBlobPart(
	ctx context.Context,
	helperClient *directUploadClient,
	sessionToken string,
	payload []byte,
	partNumber int,
) (uploadedDirectPart, error) {
	presignedURL, err := helperClient.presignPart(ctx, sessionToken, partNumber)
	if err != nil {
		return uploadedDirectPart{}, err
	}
	return helperClient.uploadPart(ctx, presignedURL, bytes.NewReader(payload), int64(len(payload)), partNumber)
}

func rawPublishLayerSize(layer modelpackports.PublishLayer) (int64, error) {
	if layer.ObjectSource != nil {
		if err := validateRawObjectSourceLayer(layer); err != nil {
			return 0, err
		}
		return layer.ObjectSource.Files[0].SizeBytes, nil
	}
	if strings.TrimSpace(layer.SourcePath) == "" {
		return 0, errors.New("source path must not be empty")
	}
	info, err := os.Stat(layer.SourcePath)
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return 0, fmt.Errorf("raw ModelPack layer %q must point to a regular file", layer.SourcePath)
	}
	return info.Size(), nil
}

func readPublishLayerChunk(
	ctx context.Context,
	layer modelpackports.PublishLayer,
	offset int64,
	length int64,
) ([]byte, error) {
	body, err := openPublishLayerRange(ctx, layer, offset, length)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	payload, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) != length {
		return nil, fmt.Errorf("short publish layer read at offset %d: got %d bytes, want %d", offset, len(payload), length)
	}
	return payload, nil
}

func hashPublishLayerRange(
	ctx context.Context,
	hasher hash.Hash,
	layer modelpackports.PublishLayer,
	offset int64,
	length int64,
) error {
	if length <= 0 {
		return nil
	}
	body, err := openPublishLayerRange(ctx, layer, offset, length)
	if err != nil {
		return err
	}
	defer body.Close()
	_, err = io.Copy(hasher, body)
	return err
}

func marshalDirectUploadDigestState(hasher hash.Hash) ([]byte, error) {
	marshaler, ok := hasher.(encoding.BinaryMarshaler)
	if !ok {
		return nil, errors.New("direct upload digest hasher does not support binary marshaling")
	}
	return marshaler.MarshalBinary()
}

func restoreDirectUploadDigestState(payload []byte) (hash.Hash, error) {
	hasher := sha256.New()
	if len(payload) == 0 {
		return hasher, nil
	}
	unmarshaler, ok := hasher.(encoding.BinaryUnmarshaler)
	if !ok {
		return nil, errors.New("direct upload digest hasher does not support binary unmarshaling")
	}
	if err := unmarshaler.UnmarshalBinary(payload); err != nil {
		return nil, err
	}
	return hasher, nil
}
