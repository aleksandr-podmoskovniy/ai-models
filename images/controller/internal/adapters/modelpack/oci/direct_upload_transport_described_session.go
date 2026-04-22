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
	"fmt"
	"net/http"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

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
