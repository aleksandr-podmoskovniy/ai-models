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

package modelsource

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

var (
	ErrUnsupportedURLScheme = errors.New("unsupported source URL scheme")
	ErrUnsupportedURLHost   = errors.New("unsupported source URL host")
	ErrUnsupportedURLPath   = errors.New("unsupported source URL path")
)

func DetectType(source modelsv1alpha1.ModelSourceSpec) (modelsv1alpha1.ModelSourceType, error) {
	switch {
	case source.Upload != nil && strings.TrimSpace(source.URL) != "":
		return "", fmt.Errorf("exactly one of source.url or source.upload must be specified")
	case source.Upload != nil:
		return modelsv1alpha1.ModelSourceTypeUpload, nil
	case strings.TrimSpace(source.URL) == "":
		return "", fmt.Errorf("source.url or source.upload must be specified")
	default:
		return DetectRemoteType(source.URL)
	}
}

func DetectRemoteType(rawURL string) (modelsv1alpha1.ModelSourceType, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("%w %q", ErrUnsupportedURLScheme, parsed.Scheme)
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "huggingface.co", "www.huggingface.co", "hf.co":
		return modelsv1alpha1.ModelSourceTypeHuggingFace, nil
	case "ollama.com", "www.ollama.com":
		if _, _, err := parseOllamaLibraryPath(parsed.Path); err != nil {
			return "", err
		}
		return modelsv1alpha1.ModelSourceTypeOllama, nil
	default:
		return "", fmt.Errorf("%w %q", ErrUnsupportedURLHost, parsed.Hostname())
	}
}

func IsUnsupportedRemoteError(err error) bool {
	return errors.Is(err, ErrUnsupportedURLScheme) ||
		errors.Is(err, ErrUnsupportedURLHost) ||
		errors.Is(err, ErrUnsupportedURLPath)
}

func ParseHuggingFaceURL(rawURL string) (string, string, error) {
	sourceType, err := DetectRemoteType(rawURL)
	if err != nil {
		return "", "", err
	}
	if sourceType != modelsv1alpha1.ModelSourceTypeHuggingFace {
		return "", "", fmt.Errorf("source URL is not a Hugging Face URL")
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", err
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 {
		return "", "", fmt.Errorf("huggingface URL must contain owner/repo")
	}

	repoID := segments[0] + "/" + segments[1]
	revision := strings.TrimSpace(parsed.Query().Get("revision"))
	if len(segments) >= 4 {
		switch segments[2] {
		case "tree", "blob", "resolve":
			revision = strings.TrimSpace(segments[3])
		}
	}

	return repoID, revision, nil
}

func ParseOllamaLibraryURL(rawURL string) (string, string, error) {
	sourceType, err := DetectRemoteType(rawURL)
	if err != nil {
		return "", "", err
	}
	if sourceType != modelsv1alpha1.ModelSourceTypeOllama {
		return "", "", fmt.Errorf("source URL is not an Ollama library URL")
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", err
	}
	return parseOllamaLibraryPath(parsed.Path)
}

func parseOllamaLibraryPath(path string) (string, string, error) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) != 2 || segments[0] != "library" || strings.TrimSpace(segments[1]) == "" {
		return "", "", fmt.Errorf("%w %q", ErrUnsupportedURLPath, path)
	}

	reference := strings.TrimSpace(segments[1])
	name, tag, found := strings.Cut(reference, ":")
	name = strings.TrimSpace(name)
	tag = strings.TrimSpace(tag)
	if name == "" || (found && tag == "") {
		return "", "", fmt.Errorf("%w %q", ErrUnsupportedURLPath, path)
	}

	return name, tag, nil
}
