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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

func (s *Service) verifyUploadedObject(ctx context.Context, objectKey string) (sealedUpload, error) {
	reader, err := s.backend.Reader(ctx, strings.TrimSpace(objectKey), 0)
	if err != nil {
		return sealedUpload{}, fmt.Errorf("failed to open uploaded object for verification: %w", err)
	}

	hasher := sha256.New()
	sizeBytes, copyErr := io.Copy(hasher, reader)
	closeErr := reader.Close()
	switch {
	case copyErr != nil:
		return sealedUpload{}, fmt.Errorf("failed to read uploaded object for verification: %w", copyErr)
	case closeErr != nil:
		return sealedUpload{}, fmt.Errorf("failed to close uploaded object reader: %w", closeErr)
	}

	return sealedUpload{
		Digest:    "sha256:" + hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes: sizeBytes,
	}, nil
}
