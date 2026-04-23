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
	"log"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

const sha256DigestBytes = sha256.Size

func (s *Service) verifyUploadedObject(ctx context.Context, objectKey string) (sealedUpload, error) {
	if sealed, ok, err := s.verifyUploadedObjectFromBackendAttributes(ctx, objectKey); err != nil {
		log.Printf("direct upload trusted backend digest lookup failed objectKey=%q error=%v; falling back to object read", strings.TrimSpace(objectKey), err)
	} else if ok {
		log.Printf("direct upload trusted backend full-object sha256 used objectKey=%q digest=%q sizeBytes=%d", strings.TrimSpace(objectKey), sealed.Digest, sealed.SizeBytes)
		return sealed, nil
	}
	return s.verifyUploadedObjectByReading(ctx, objectKey)
}

func (s *Service) verifyUploadedObjectFromBackendAttributes(ctx context.Context, objectKey string) (sealedUpload, bool, error) {
	attributes, err := s.backend.ObjectAttributes(ctx, strings.TrimSpace(objectKey))
	if err != nil {
		return sealedUpload{}, false, err
	}
	if attributes.SizeBytes < 0 {
		return sealedUpload{}, false, fmt.Errorf("trusted backend sizeBytes must not be negative")
	}
	dgst := strings.TrimSpace(attributes.SHA256Digest)
	if dgst == "" {
		return sealedUpload{}, false, nil
	}
	parsedDigest, err := digest.Parse(dgst)
	if err != nil || parsedDigest.Algorithm().String() != "sha256" {
		return sealedUpload{}, false, nil
	}
	return sealedUpload{
		Digest:    parsedDigest.String(),
		SizeBytes: attributes.SizeBytes,
	}, true, nil
}

func (s *Service) verifyUploadedObjectByReading(ctx context.Context, objectKey string) (sealedUpload, error) {
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
