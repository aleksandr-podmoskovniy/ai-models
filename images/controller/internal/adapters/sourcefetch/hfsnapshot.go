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
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type huggingFaceHTTPSnapshotDownloader struct {
	BaseURL    string
	HTTPClient *http.Client
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
