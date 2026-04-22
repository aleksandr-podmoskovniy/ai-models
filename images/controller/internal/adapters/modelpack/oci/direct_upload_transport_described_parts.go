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

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

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
