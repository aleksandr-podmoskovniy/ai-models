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
	"strconv"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type blobRangeMode string

const (
	blobRangePartial  blobRangeMode = "partial"
	blobRangeFullBody blobRangeMode = "full-body"
)

func FetchBlobRange(
	ctx context.Context,
	client *http.Client,
	reference string,
	digest string,
	auth modelpackports.RegistryAuth,
	offset int64,
	length int64,
) ([]byte, blobRangeMode, error) {
	if offset < 0 || length <= 0 {
		return nil, "", fmt.Errorf("invalid blob range offset=%d length=%d", offset, length)
	}
	if offset > maxInt64-length+1 {
		return nil, "", fmt.Errorf("invalid blob range offset=%d length=%d overflows int64", offset, length)
	}
	blobURL, err := RegistryBlobURL(reference, digest)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query remote blob range: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPartialContent:
		if err := validateContentRange(resp.Header.Get("Content-Range"), offset, length); err != nil {
			return nil, "", err
		}
		payload, err := readExactResponseBody(resp.Body, length)
		return payload, blobRangePartial, err
	case http.StatusOK:
		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read full blob fallback body: %w", err)
		}
		if offset+length > int64(len(payload)) {
			return nil, "", fmt.Errorf("full blob fallback body size %d does not contain requested range offset=%d length=%d", len(payload), offset, length)
		}
		return payload, blobRangeFullBody, nil
	case http.StatusRequestedRangeNotSatisfiable:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("remote blob range is not satisfiable: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("failed to query remote blob range: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func validateContentRange(header string, offset, length int64) error {
	header = strings.TrimSpace(header)
	prefix := fmt.Sprintf("bytes %d-%d/", offset, offset+length-1)
	if !strings.HasPrefix(header, prefix) {
		return fmt.Errorf("unexpected Content-Range %q for offset=%d length=%d", header, offset, length)
	}
	total := strings.TrimPrefix(header, prefix)
	if total == "*" {
		return nil
	}
	if _, err := strconv.ParseInt(total, 10, 64); err != nil {
		return fmt.Errorf("unexpected Content-Range %q: %w", header, err)
	}
	return nil
}

func readExactResponseBody(reader io.Reader, length int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, length+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read remote blob range: %w", err)
	}
	if int64(len(payload)) != length {
		return nil, fmt.Errorf("remote blob range length %d does not match expected %d", len(payload), length)
	}
	return payload, nil
}
