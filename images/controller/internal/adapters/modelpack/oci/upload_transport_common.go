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
