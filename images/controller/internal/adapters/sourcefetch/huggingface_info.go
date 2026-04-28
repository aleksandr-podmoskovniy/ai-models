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
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
)

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
		ID:           firstNonEmpty(payload.ID, repoID),
		SHA:          strings.TrimSpace(payload.SHA),
		PipelineTag:  strings.TrimSpace(payload.PipelineTag),
		DeclaredTask: declaredHuggingFaceTask(payload),
		License:      strings.TrimSpace(payload.CardData.License),
		Files:        files,
	}, nil
}

func declaredHuggingFaceTask(payload huggingFaceAPIResponse) string {
	if task := declaredHuggingFaceModelIndexTask(payload.CardData.ModelIndex); task != "" {
		return task
	}
	return normalizeHuggingFaceTaskType(payload.PipelineTag)
}

func declaredHuggingFaceModelIndexTask(index []huggingFaceModelIndex) string {
	declared := ""
	for _, model := range index {
		for _, result := range model.Results {
			task := normalizeHuggingFaceTaskType(result.Task.Type)
			if task == "" {
				continue
			}
			if declared == "" {
				declared = task
				continue
			}
			if declared != task {
				return ""
			}
		}
	}
	return declared
}

func normalizeHuggingFaceTaskType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")
	switch normalized {
	case "reranking", "rerank", "text-ranking":
		return "rerank"
	case "sentence-similarity", "semantic-textual-similarity", "sts":
		return "sentence-similarity"
	case "text-generation", "text2text-generation", "summarization", "conversational",
		"feature-extraction", "embeddings", "text-embeddings-inference",
		"automatic-speech-recognition", "speech-to-text", "text-to-speech",
		"text-to-audio", "text-to-music", "audio-generation", "translation",
		"image-classification", "zero-shot-image-classification",
		"object-detection", "zero-shot-object-detection", "image-segmentation",
		"image-to-text", "image-text-to-text", "visual-question-answering",
		"document-question-answering", "text-to-image", "image-generation",
		"unconditional-image-generation", "image-to-image", "image-variation",
		"inpainting", "image-inpainting", "text-to-video", "video-generation",
		"image-to-video", "video-to-video":
		return normalized
	default:
		return ""
	}
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

func cleanRemoteRelativePath(path string) (string, error) {
	return archiveio.RelativePath(strings.ReplaceAll(path, "\\", "/"))
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
