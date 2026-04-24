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
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

func (s *Service) sealedBlobExists(ctx context.Context, blobKey string) (bool, error) {
	if exists, err := s.backend.ObjectExists(ctx, blobKey); err != nil || exists {
		return exists, err
	}
	return s.backend.ObjectExists(ctx, sealedblob.MetadataPath(blobKey))
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

func (s *Service) cleanupSealedUpload(ctx context.Context, blobKey, physicalPath string) error {
	var cleanupErrs []error
	if err := s.backend.DeleteObject(ctx, strings.TrimSpace(physicalPath)); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}
	if err := s.backend.DeleteObject(ctx, sealedblob.MetadataPath(blobKey)); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}
	return errors.Join(cleanupErrs...)
}
