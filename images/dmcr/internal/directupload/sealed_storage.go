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

package directupload

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

type sealedBlobState struct {
	exists               bool
	deleteUploadedObject bool
}

func (s *Service) sealedBlobState(ctx context.Context, blobKey string, sealed sealedUpload, uploadedPhysicalPath string) (sealedBlobState, error) {
	if exists, err := s.backend.ObjectExists(ctx, blobKey); err != nil || exists {
		return sealedBlobState{exists: exists, deleteUploadedObject: exists}, err
	}
	metadata, exists, err := s.readSealedBlobMetadata(ctx, blobKey)
	if err != nil || !exists {
		return sealedBlobState{exists: exists}, err
	}
	if strings.TrimSpace(metadata.Digest) != strings.TrimSpace(sealed.Digest) {
		return sealedBlobState{}, fmt.Errorf("sealed metadata digest %q does not match digest %q", metadata.Digest, sealed.Digest)
	}
	if metadata.SizeBytes != sealed.SizeBytes {
		return sealedBlobState{}, fmt.Errorf("sealed metadata sizeBytes %d does not match sizeBytes %d", metadata.SizeBytes, sealed.SizeBytes)
	}
	return sealedBlobState{
		exists:               true,
		deleteUploadedObject: strings.TrimSpace(metadata.PhysicalPath) != strings.TrimSpace(uploadedPhysicalPath),
	}, nil
}

func (s *Service) readSealedBlobMetadata(ctx context.Context, blobKey string) (sealedblob.Metadata, bool, error) {
	metadataKey := sealedblob.MetadataPath(blobKey)
	exists, err := s.backend.ObjectExists(ctx, metadataKey)
	if err != nil || !exists {
		return sealedblob.Metadata{}, exists, err
	}
	reader, err := s.backend.Reader(ctx, metadataKey, 0)
	if err != nil {
		return sealedblob.Metadata{}, true, err
	}
	payload, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return sealedblob.Metadata{}, true, readErr
	}
	if closeErr != nil {
		return sealedblob.Metadata{}, true, closeErr
	}
	metadata, err := sealedblob.Unmarshal(payload)
	return metadata, true, err
}

func (s *Service) writeSealedBlobMetadata(ctx context.Context, blobKey string, sealed sealedUpload, physicalPath string) error {
	payload, err := sealedblob.Marshal(sealedblob.Metadata{
		Version:      sealedblob.MetadataVersion,
		Digest:       sealed.Digest,
		PhysicalPath: strings.TrimSpace(physicalPath),
		SizeBytes:    sealed.SizeBytes,
	})
	if err != nil {
		return err
	}
	return s.backend.PutContent(ctx, sealedblob.MetadataPath(blobKey), payload)
}
