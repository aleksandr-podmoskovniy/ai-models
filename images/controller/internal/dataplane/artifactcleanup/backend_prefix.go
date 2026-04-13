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

package artifactcleanup

import (
	"path"
	"slices"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func backendObjectStoragePrefixes(handle cleanuphandle.Handle) []string {
	result := make([]string, 0, 2)
	if prefix := backendRepositoryMetadataPrefix(handle); prefix != "" {
		result = append(result, prefix)
	}
	if prefix := backendSourceMirrorPrefix(handle); prefix != "" {
		result = append(result, prefix)
	}
	return slices.Compact(result)
}

func backendRepositoryMetadataPrefix(handle cleanuphandle.Handle) string {
	if handle.Backend == nil {
		return ""
	}
	if value := strings.Trim(strings.TrimSpace(handle.Backend.RepositoryMetadataPrefix), "/"); value != "" {
		return value
	}

	repository := repositoryPathFromOCIReference(handle.Backend.Reference)
	if repository == "" {
		return ""
	}
	return path.Join("dmcr", "docker", "registry", "v2", "repositories", repository)
}

func backendSourceMirrorPrefix(handle cleanuphandle.Handle) string {
	if handle.Backend == nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(handle.Backend.SourceMirrorPrefix), "/")
}

func repositoryPathFromOCIReference(reference string) string {
	cleanReference := strings.TrimSpace(strings.SplitN(reference, "@", 2)[0])
	registry, repository, found := strings.Cut(cleanReference, "/")
	if !found || strings.TrimSpace(registry) == "" {
		return ""
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return ""
	}
	repositoryPart := repository[strings.LastIndex(repository, "/")+1:]
	if strings.Contains(repositoryPart, ":") {
		repository = repository[:strings.LastIndex(repository, ":")]
	}
	return strings.Trim(repository, "/")
}
