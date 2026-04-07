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

const huggingFaceBaseURL = "https://huggingface.co"

type HuggingFaceInfo struct {
	ID          string
	SHA         string
	Private     bool
	Gated       bool
	Downloads   int64
	Likes       int64
	LibraryName string
	PipelineTag string
	Tags        []string
	License     string
	BaseModel   string
	Files       []string
}

type huggingFaceAPIResponse struct {
	ID          string                   `json:"id"`
	SHA         string                   `json:"sha"`
	Private     bool                     `json:"private"`
	Gated       any                      `json:"gated"`
	Downloads   int64                    `json:"downloads"`
	Likes       int64                    `json:"likes"`
	LibraryName string                   `json:"library_name"`
	PipelineTag string                   `json:"pipeline_tag"`
	Tags        []string                 `json:"tags"`
	CardData    map[string]any           `json:"cardData"`
	Siblings    []huggingFaceSiblingInfo `json:"siblings"`
}

type huggingFaceSiblingInfo struct {
	Path string `json:"rfilename"`
}

func DownloadHuggingFaceFiles(
	ctx context.Context,
	repoID, revision, token, destination string,
	files []string,
) error {
	for _, path := range files {
		if err := downloadHuggingFaceFile(ctx, repoID, revision, token, path, destination); err != nil {
			return err
		}
	}
	return nil
}

func FetchHuggingFaceInfo(ctx context.Context, repoID, revision, token string) (HuggingFaceInfo, error) {
	endpoint, err := huggingFaceInfoURL(repoID, revision)
	if err != nil {
		return HuggingFaceInfo{}, err
	}

	response, err := doGET(ctx, nil, endpoint, bearerAuthHeaders(token))
	if err != nil {
		return HuggingFaceInfo{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return HuggingFaceInfo{}, unexpectedStatusError(response, "huggingface model info request")
	}

	var payload huggingFaceAPIResponse
	if err := decodeJSONResponse(response, &payload); err != nil {
		return HuggingFaceInfo{}, err
	}

	files := make([]string, 0, len(payload.Siblings))
	for _, item := range payload.Siblings {
		if path := strings.TrimSpace(item.Path); path != "" {
			files = append(files, path)
		}
	}

	return HuggingFaceInfo{
		ID:          firstNonEmpty(payload.ID, repoID),
		SHA:         strings.TrimSpace(payload.SHA),
		Private:     payload.Private,
		Gated:       payload.Gated != nil && payload.Gated != false,
		Downloads:   payload.Downloads,
		Likes:       payload.Likes,
		LibraryName: strings.TrimSpace(payload.LibraryName),
		PipelineTag: strings.TrimSpace(payload.PipelineTag),
		Tags:        payload.Tags,
		License:     stringValue(payload.CardData["license"]),
		BaseModel:   stringValue(payload.CardData["base_model"]),
		Files:       files,
	}, nil
}

func downloadHuggingFaceFile(ctx context.Context, repoID, revision, token, path, destination string) error {
	endpoint, err := huggingFaceResolveURL(repoID, revision, path)
	if err != nil {
		return err
	}
	response, err := doGET(ctx, nil, endpoint, bearerAuthHeaders(token))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return unexpectedStatusError(response, fmt.Sprintf("huggingface download for %q", path))
	}

	target := filepath.Join(destination, filepath.Clean(path))
	return writeResponseBody(target, response.Body)
}

func huggingFaceInfoURL(repoID, revision string) (string, error) {
	repositoryPath, err := huggingFaceRepoPath(repoID)
	if err != nil {
		return "", err
	}
	endpoint := huggingFaceBaseURL + "/api/models/" + repositoryPath
	if strings.TrimSpace(revision) == "" {
		return endpoint, nil
	}
	return endpoint + "?revision=" + url.QueryEscape(strings.TrimSpace(revision)), nil
}

func huggingFaceResolveURL(repoID, revision, path string) (string, error) {
	repositoryPath, err := huggingFaceRepoPath(repoID)
	if err != nil {
		return "", err
	}
	trimmedPath := strings.Trim(strings.TrimSpace(path), "/")
	if trimmedPath == "" {
		return "", errors.New("huggingface file path must not be empty")
	}
	resolvedRevision := strings.TrimSpace(revision)
	if resolvedRevision == "" {
		resolvedRevision = "main"
	}
	return huggingFaceBaseURL + "/" + repositoryPath + "/resolve/" + url.PathEscape(resolvedRevision) + "/" + escapeHuggingFaceFilePath(trimmedPath) + "?download=1", nil
}

func huggingFaceRepoPath(repoID string) (string, error) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(repoID), "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("huggingface repo ID must be owner/name, got %q", repoID)
	}
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean == "" {
			return "", fmt.Errorf("huggingface repo ID contains an empty path segment: %q", repoID)
		}
		escaped = append(escaped, url.PathEscape(clean))
	}
	return strings.Join(escaped, "/"), nil
}

func escapeHuggingFaceFilePath(path string) string {
	parts := strings.Split(path, "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return strings.Join(escaped, "/")
}

func ResolveHuggingFaceRevision(info HuggingFaceInfo, requested string) string {
	if strings.TrimSpace(info.SHA) != "" {
		return strings.TrimSpace(info.SHA)
	}
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	return "main"
}

func stringValue(value any) string {
	typed, _ := value.(string)
	return strings.TrimSpace(typed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
