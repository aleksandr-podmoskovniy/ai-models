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
	"io"
	"net/http"
	"sort"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const defaultBlobUploadRecoveryAttempts = 5

var blobUploadRecoveryAttempts = defaultBlobUploadRecoveryAttempts

func blobExists(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	digest string,
) (bool, error) {
	blobURL, err := RegistryBlobURL(reference, digest)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, blobURL, nil)
	if err != nil {
		return false, err
	}
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to query remote blob existence: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return false, fmt.Errorf("failed to query remote blob existence: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func resolvedResponseLocation(baseURL, location string) (string, error) {
	if location == "" {
		return baseURL, nil
	}
	return resolveUploadLocation(baseURL, location)
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
