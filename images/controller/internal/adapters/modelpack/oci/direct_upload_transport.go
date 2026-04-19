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

func pushLayerDirectToBackingStorage(
	ctx context.Context,
	_ *http.Client,
	input modelpackports.PublishInput,
	auth modelpackports.RegistryAuth,
	layer modelpackports.PublishLayer,
	descriptor publishLayerDescriptor,
) error {
	parsedReference, err := parseOCIReference(input.ArtifactURI)
	if err != nil {
		return err
	}
	helperClient, err := newDirectUploadClient(input, auth)
	if err != nil {
		return err
	}

	session, err := helperClient.start(ctx, parsedReference.Repository, descriptor.Digest)
	if err != nil {
		return err
	}
	if session.Complete {
		return nil
	}
	if session.PartSizeBytes <= 0 {
		return errors.New("DMCR direct upload session returned non-positive part size")
	}

	completeStarted := false
	completed := false
	defer func() {
		if completed || completeStarted {
			return
		}
		_ = helperClient.abort(ctx, session.SessionToken)
	}()

	uploadedParts, err := uploadDirectBlobParts(ctx, helperClient, layer, descriptor.Size, session)
	if err != nil {
		return err
	}

	completeStarted = true
	if err := helperClient.complete(ctx, session.SessionToken, uploadedParts); err != nil {
		return err
	}
	completed = true
	return nil
}

func uploadDirectBlobParts(
	ctx context.Context,
	helperClient *directUploadClient,
	layer modelpackports.PublishLayer,
	totalSize int64,
	session directUploadSession,
) ([]uploadedDirectPart, error) {
	uploadedParts := make([]uploadedDirectPart, 0)
	offset := int64(0)
	partNumber := 1
	recoveries := 0

	for offset < totalSize {
		uploadedPart, err := uploadDirectBlobPart(ctx, helperClient, session, layer, offset, partNumber, totalSize)
		if err == nil {
			uploadedParts = append(uploadedParts, uploadedPart)
			offset += uploadedPart.SizeBytes
			partNumber++
			recoveries = 0
			continue
		}

		nextParts, nextOffset, nextPartNumber, recoveryErr := recoverDirectBlobUpload(ctx, helperClient, session.SessionToken, err)
		if recoveryErr != nil {
			return nil, recoveryErr
		}
		uploadedParts = nextParts
		offset = nextOffset
		partNumber = nextPartNumber
		recoveries++
		if recoveries > blobUploadRecoveryAttempts {
			return nil, err
		}
	}

	return uploadedParts, nil
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
