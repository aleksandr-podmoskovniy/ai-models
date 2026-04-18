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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const (
	defaultBlobUploadChunkSize        int64 = 8 << 20
	defaultBlobUploadRecoveryAttempts       = 5
)

var (
	blobUploadChunkSize        = defaultBlobUploadChunkSize
	blobUploadRecoveryAttempts = defaultBlobUploadRecoveryAttempts
	errUploadSessionNotFound   = errors.New("modelpack blob upload session not found")
)

type uploadSession struct {
	Location       string
	ChunkMinLength int64
}

type uploadStatus struct {
	Location string
	Offset   int64
}

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

func initiateBlobUploadSession(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
) (uploadSession, error) {
	uploadURL, err := initiateBlobUpload(ctx, client, reference, auth)
	if err != nil {
		return uploadSession{}, err
	}
	return uploadSession{
		Location:       uploadURL.Location,
		ChunkMinLength: uploadURL.ChunkMinLength,
	}, nil
}

func uploadBlobChunkAt(
	ctx context.Context,
	client *http.Client,
	uploadURL string,
	auth modelpackports.RegistryAuth,
	layer modelpackports.PublishLayer,
	offset int64,
	length int64,
) (uploadStatus, error) {
	body, err := openPublishLayerRange(ctx, layer, offset, length)
	if err != nil {
		return uploadStatus{}, err
	}
	defer body.Close()

	rangeEnd := offset + length - 1
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, uploadURL, body)
	if err != nil {
		return uploadStatus{}, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Range", fmt.Sprintf("%d-%d", offset, rangeEnd))
	req.ContentLength = length
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return uploadStatus{}, fmt.Errorf("failed to stream modelpack blob chunk: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return uploadStatus{}, fmt.Errorf("failed to stream modelpack blob chunk: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	location, err := resolvedResponseLocation(uploadURL, resp.Header.Get("Location"))
	if err != nil {
		return uploadStatus{}, err
	}
	nextOffset, err := nextOffsetFromRange(resp.Header.Get("Range"))
	if err != nil {
		return uploadStatus{}, err
	}
	if nextOffset == 0 {
		nextOffset = rangeEnd + 1
	}

	return uploadStatus{
		Location: location,
		Offset:   nextOffset,
	}, nil
}

func getUploadStatus(
	ctx context.Context,
	client *http.Client,
	uploadURL string,
	auth modelpackports.RegistryAuth,
) (uploadStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uploadURL, nil)
	if err != nil {
		return uploadStatus{}, err
	}
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return uploadStatus{}, fmt.Errorf("failed to query modelpack upload status: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		location, err := resolvedResponseLocation(uploadURL, resp.Header.Get("Location"))
		if err != nil {
			return uploadStatus{}, err
		}
		offset, err := nextOffsetFromRange(resp.Header.Get("Range"))
		if err != nil {
			return uploadStatus{}, err
		}
		return uploadStatus{
			Location: location,
			Offset:   offset,
		}, nil
	case http.StatusNotFound:
		return uploadStatus{}, errUploadSessionNotFound
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return uploadStatus{}, fmt.Errorf("failed to query modelpack upload status: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func nextUploadChunkLength(offset int64, total int64, minChunkLength int64) int64 {
	remaining := total - offset
	if remaining <= 0 {
		return 0
	}

	chunkLength := blobUploadChunkSize
	if chunkLength <= 0 {
		chunkLength = defaultBlobUploadChunkSize
	}
	if minChunkLength > chunkLength && remaining > minChunkLength {
		chunkLength = minChunkLength
	}
	if remaining < chunkLength {
		return remaining
	}
	return chunkLength
}

func nextOffsetFromRange(header string) (int64, error) {
	cleanHeader := strings.TrimSpace(header)
	if cleanHeader == "" {
		return 0, nil
	}
	parts := strings.Split(cleanHeader, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid upload Range header %q", header)
	}
	end, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid upload Range header %q: %w", header, err)
	}
	return end + 1, nil
}

func resolvedResponseLocation(baseURL, location string) (string, error) {
	if strings.TrimSpace(location) == "" {
		return strings.TrimSpace(baseURL), nil
	}
	return resolveUploadLocation(baseURL, location)
}
