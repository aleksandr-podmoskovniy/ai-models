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

package sourcefetch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

type huggingFaceSnapshotDownloadInput struct {
	RepoID      string
	Revision    string
	Token       string
	Files       []string
	SnapshotDir string
}

type huggingFaceSnapshotDownloader interface {
	Download(context.Context, huggingFaceSnapshotDownloadInput) error
}

var newHuggingFaceSnapshotDownloader = func() huggingFaceSnapshotDownloader {
	return &huggingFaceHTTPSnapshotDownloader{
		BaseURL:    huggingFaceBaseURL,
		HTTPClient: http.DefaultClient,
	}
}

type huggingFaceHTTPSnapshotDownloader struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (d *huggingFaceHTTPSnapshotDownloader) Download(ctx context.Context, input huggingFaceSnapshotDownloadInput) error {
	if err := validateHuggingFaceSnapshotDownloadInput(input); err != nil {
		return err
	}

	headers := bearerAuthHeaders(input.Token)
	client := d.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	for _, filePath := range input.Files {
		cleanPath, err := cleanRemoteRelativePath(filePath)
		if err != nil {
			return err
		}

		sourceURL, err := d.resolveURL(input.RepoID, input.Revision, cleanPath)
		if err != nil {
			return err
		}
		response, err := doGET(ctx, client, sourceURL, headers)
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			defer response.Body.Close()
			return unexpectedStatusError(response, "huggingface snapshot download")
		}

		target := filepath.Join(strings.TrimSpace(input.SnapshotDir), cleanPath)
		writeErr := writeResponseBody(target, response.Body)
		closeErr := response.Body.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}

	return nil
}

func (d *huggingFaceHTTPSnapshotDownloader) resolveURL(repoID, revision, relativePath string) (string, error) {
	repoPath, err := huggingFaceRepoPath(repoID)
	if err != nil {
		return "", err
	}
	cleanPath, err := cleanRemoteRelativePath(relativePath)
	if err != nil {
		return "", err
	}

	baseURL := strings.TrimRight(strings.TrimSpace(d.BaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(huggingFaceBaseURL, "/")
	}

	return fmt.Sprintf(
		"%s/%s/resolve/%s/%s",
		baseURL,
		repoPath,
		escapePathPreservingSeparators(revision),
		escapePathPreservingSeparators(cleanPath),
	), nil
}

func validateHuggingFaceSnapshotDownloadInput(input huggingFaceSnapshotDownloadInput) error {
	if strings.TrimSpace(input.RepoID) == "" {
		return errors.New("huggingface repo ID must not be empty")
	}
	if strings.TrimSpace(input.Revision) == "" {
		return errors.New("huggingface revision must not be empty")
	}
	if strings.TrimSpace(input.SnapshotDir) == "" {
		return errors.New("huggingface snapshot directory must not be empty")
	}
	if len(input.Files) == 0 {
		return errors.New("huggingface file selection must not be empty")
	}
	return nil
}

func escapePathPreservingSeparators(value string) string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(value), "/"), "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(part))
	}
	return strings.Join(escaped, "/")
}
