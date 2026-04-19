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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type uploadLocation struct {
	Location string
}

func uploadBlobFromReader(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	body io.Reader,
	size int64,
	digest string,
) error {
	exists, err := blobExists(ctx, client, reference, auth, digest)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	uploadURL, err := initiateBlobUpload(ctx, client, reference, auth)
	if err != nil {
		return err
	}
	parsedUploadURL, err := url.Parse(uploadURL.Location)
	if err != nil {
		return err
	}
	query := parsedUploadURL.Query()
	query.Set("digest", strings.TrimSpace(digest))
	parsedUploadURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, parsedUploadURL.String(), body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload modelpack blob: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("failed to upload modelpack blob: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

func initiateBlobUpload(ctx context.Context, client *http.Client, reference string, auth modelpackports.RegistryAuth) (uploadLocation, error) {
	parsed, err := parseOCIReference(reference)
	if err != nil {
		return uploadLocation{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.uploadURL(), nil)
	if err != nil {
		return uploadLocation{}, err
	}
	req.ContentLength = 0
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return uploadLocation{}, fmt.Errorf("failed to initiate modelpack blob upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return uploadLocation{}, fmt.Errorf("failed to initiate modelpack blob upload: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	location, err := resolvedResponseLocation(parsed.uploadURL(), resp.Header.Get("Location"))
	if err != nil {
		return uploadLocation{}, err
	}
	return uploadLocation{
		Location: location,
	}, nil
}

func putManifest(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	manifest []byte,
) error {
	manifestURL, err := RegistryManifestURL(reference)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, manifestURL, bytes.NewReader(manifest))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ManifestMediaType)
	req.ContentLength = int64(len(manifest))
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to publish modelpack manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("failed to publish modelpack manifest: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
