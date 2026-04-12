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

package v1alpha1

import (
	"fmt"
	"net/url"
	"strings"
)

func (s ModelSourceSpec) DetectType() (ModelSourceType, error) {
	switch {
	case s.Upload != nil && strings.TrimSpace(s.URL) != "":
		return "", fmt.Errorf("exactly one of source.url or source.upload must be specified")
	case s.Upload != nil:
		return ModelSourceTypeUpload, nil
	case strings.TrimSpace(s.URL) == "":
		return "", fmt.Errorf("source.url or source.upload must be specified")
	default:
		return DetectRemoteSourceType(s.URL)
	}
}

func DetectRemoteSourceType(rawURL string) (ModelSourceType, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported source URL scheme %q", parsed.Scheme)
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "huggingface.co", "www.huggingface.co", "hf.co":
		return ModelSourceTypeHuggingFace, nil
	default:
		return "", fmt.Errorf("unsupported source URL host %q", parsed.Hostname())
	}
}

func ParseHuggingFaceURL(rawURL string) (string, string, error) {
	sourceType, err := DetectRemoteSourceType(rawURL)
	if err != nil {
		return "", "", err
	}
	if sourceType != ModelSourceTypeHuggingFace {
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
