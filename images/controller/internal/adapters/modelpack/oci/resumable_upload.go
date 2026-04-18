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
	"net/http"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func pushLayerResumable(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	layer modelpackports.PublishLayer,
	descriptor publishLayerDescriptor,
) error {
	exists, err := blobExists(ctx, client, reference, auth, descriptor.Digest)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	session, err := initiateBlobUploadSession(ctx, client, reference, auth)
	if err != nil {
		return err
	}

	offset := int64(0)
	recoveries := 0
	for offset < descriptor.Size {
		advanced, err := pushLayerChunk(ctx, client, auth, layer, descriptor.Size, session, offset)
		if err == nil {
			offset = advanced.Offset
			session = advanced.Session
			recoveries = 0
			continue
		}

		recovered, recoveryErr := recoverBlobUpload(ctx, client, reference, auth, descriptor.Digest, session, err)
		if recoveryErr != nil {
			return recoveryErr
		}
		if recovered.Complete {
			return nil
		}
		offset = recovered.Offset
		session = recovered.Session
		recoveries++
		if recoveries > blobUploadRecoveryAttempts {
			return err
		}
	}

	return finalizeUploadedBlob(ctx, client, reference, auth, session.Location, descriptor.Digest)
}

type uploadProgress struct {
	Complete bool
	Session  uploadSession
	Offset   int64
}

func pushLayerChunk(
	ctx context.Context,
	client *http.Client,
	auth modelpackports.RegistryAuth,
	layer modelpackports.PublishLayer,
	totalSize int64,
	session uploadSession,
	offset int64,
) (uploadProgress, error) {
	chunkLength := nextUploadChunkLength(offset, totalSize, session.ChunkMinLength)
	status, err := uploadBlobChunkAt(ctx, client, session.Location, auth, layer, offset, chunkLength)
	if err != nil {
		return uploadProgress{}, err
	}
	return uploadProgress{
		Session: uploadSession{
			Location:       status.Location,
			ChunkMinLength: session.ChunkMinLength,
		},
		Offset: status.Offset,
	}, nil
}

func recoverBlobUpload(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	digest string,
	session uploadSession,
	cause error,
) (uploadProgress, error) {
	exists, headErr := blobExists(ctx, client, reference, auth, digest)
	if headErr == nil && exists {
		return uploadProgress{Complete: true, Session: session}, nil
	}

	status, statusErr := getUploadStatus(ctx, client, session.Location, auth)
	switch {
	case statusErr == nil:
		return uploadProgress{
			Session: uploadSession{
				Location:       status.Location,
				ChunkMinLength: session.ChunkMinLength,
			},
			Offset: status.Offset,
		}, nil
	case errors.Is(statusErr, errUploadSessionNotFound):
		restarted, restartErr := initiateBlobUploadSession(ctx, client, reference, auth)
		if restartErr != nil {
			return uploadProgress{}, errors.Join(cause, restartErr)
		}
		return uploadProgress{Session: restarted, Offset: 0}, nil
	default:
		return uploadProgress{}, errors.Join(cause, statusErr)
	}
}

func finalizeUploadedBlob(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	uploadURL string,
	digest string,
) error {
	if err := finalizeBlobUpload(ctx, client, uploadURL, auth, digest); err != nil {
		exists, headErr := blobExists(ctx, client, reference, auth, digest)
		if headErr == nil && exists {
			return nil
		}
		if headErr != nil {
			return errors.Join(err, headErr)
		}
		return err
	}
	return nil
}
