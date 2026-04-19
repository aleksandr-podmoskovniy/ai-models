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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type directUploadSession struct {
	Complete      bool
	SessionToken  string
	PartSizeBytes int64
}

type uploadedDirectPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	SizeBytes  int64  `json:"sizeBytes"`
}

type directUploadClient struct {
	apiClient    *http.Client
	uploadClient *http.Client
	endpoint     string
	auth         modelpackports.RegistryAuth
}

type startDirectUploadRequest struct {
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
}

type startDirectUploadResponse struct {
	Complete      bool   `json:"complete"`
	SessionToken  string `json:"sessionToken"`
	PartSizeBytes int64  `json:"partSizeBytes"`
}

type presignDirectUploadPartRequest struct {
	SessionToken string `json:"sessionToken"`
	PartNumber   int    `json:"partNumber"`
}

type presignDirectUploadPartResponse struct {
	URL string `json:"url"`
}

type listDirectUploadPartsResponse struct {
	Parts []uploadedDirectPart `json:"parts"`
}

type completeDirectUploadRequest struct {
	SessionToken string               `json:"sessionToken"`
	Parts        []uploadedDirectPart `json:"parts"`
}

type abortDirectUploadRequest struct {
	SessionToken string `json:"sessionToken"`
}

func newDirectUploadClient(
	input modelpackports.PublishInput,
	auth modelpackports.RegistryAuth,
) (*directUploadClient, error) {
	apiClient, err := RegistryHTTPClient(auth)
	if err != nil {
		return nil, err
	}
	uploadClient, err := tlsHTTPClient(input.DirectUploadCAFile, input.DirectUploadInsecure, "direct upload")
	if err != nil {
		return nil, err
	}
	return &directUploadClient{
		apiClient:    apiClient,
		uploadClient: uploadClient,
		endpoint:     strings.TrimRight(strings.TrimSpace(input.DirectUploadEndpoint), "/"),
		auth:         auth,
	}, nil
}

func (c *directUploadClient) start(
	ctx context.Context,
	repository string,
	digest string,
) (directUploadSession, error) {
	var response startDirectUploadResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/blob-uploads", startDirectUploadRequest{
		Repository: strings.TrimSpace(repository),
		Digest:     strings.TrimSpace(digest),
	}, &response); err != nil {
		return directUploadSession{}, err
	}
	return directUploadSession{
		Complete:      response.Complete,
		SessionToken:  strings.TrimSpace(response.SessionToken),
		PartSizeBytes: response.PartSizeBytes,
	}, nil
}

func (c *directUploadClient) presignPart(
	ctx context.Context,
	sessionToken string,
	partNumber int,
) (string, error) {
	var response presignDirectUploadPartResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/blob-uploads/presign-part", presignDirectUploadPartRequest{
		SessionToken: strings.TrimSpace(sessionToken),
		PartNumber:   partNumber,
	}, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.URL) == "" {
		return "", fmt.Errorf("direct upload presign response is missing URL")
	}
	return strings.TrimSpace(response.URL), nil
}

func (c *directUploadClient) listParts(ctx context.Context, sessionToken string) ([]uploadedDirectPart, error) {
	requestPath := "/v1/blob-uploads/parts?sessionToken=" + url.QueryEscape(strings.TrimSpace(sessionToken))
	var response listDirectUploadPartsResponse
	if err := c.doJSON(ctx, http.MethodGet, requestPath, nil, &response); err != nil {
		return nil, err
	}
	return normalizeUploadedDirectParts(response.Parts)
}

func (c *directUploadClient) complete(
	ctx context.Context,
	sessionToken string,
	parts []uploadedDirectPart,
) error {
	return c.doJSON(ctx, http.MethodPost, "/v1/blob-uploads/complete", completeDirectUploadRequest{
		SessionToken: strings.TrimSpace(sessionToken),
		Parts:        parts,
	}, nil)
}

func (c *directUploadClient) abort(ctx context.Context, sessionToken string) error {
	return c.doJSON(ctx, http.MethodPost, "/v1/blob-uploads/abort", abortDirectUploadRequest{
		SessionToken: strings.TrimSpace(sessionToken),
	}, nil)
}

func (c *directUploadClient) uploadPart(
	ctx context.Context,
	presignedURL string,
	body io.Reader,
	length int64,
	partNumber int,
) (uploadedDirectPart, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimSpace(presignedURL), body)
	if err != nil {
		return uploadedDirectPart{}, err
	}
	req.ContentLength = length

	resp, err := c.uploadClient.Do(req)
	if err != nil {
		return uploadedDirectPart{}, fmt.Errorf("failed to upload direct blob part: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return uploadedDirectPart{}, fmt.Errorf("failed to upload direct blob part: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	etag := strings.Trim(strings.TrimSpace(resp.Header.Get("ETag")), "\"")
	if etag == "" {
		return uploadedDirectPart{}, fmt.Errorf("direct blob part upload response is missing ETag header")
	}
	return uploadedDirectPart{
		PartNumber: partNumber,
		ETag:       etag,
		SizeBytes:  length,
	}, nil
}

func (c *directUploadClient) doJSON(
	ctx context.Context,
	method string,
	requestPath string,
	requestBody any,
	responseBody any,
) error {
	endpoint := c.endpoint + requestPath
	var body io.Reader
	if requestBody != nil {
		payload, err := jsonMarshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(c.auth.Username, c.auth.Password)

	resp, err := c.apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call DMCR direct upload API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		responsePayload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("DMCR direct upload API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responsePayload)))
	}
	if responseBody == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
		return fmt.Errorf("failed to decode DMCR direct upload API response: %w", err)
	}
	return nil
}
