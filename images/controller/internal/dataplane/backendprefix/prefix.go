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

package backendprefix

import (
	"path"
	"strings"
)

func RepositoryMetadataPrefixFromReference(reference string) string {
	repository := RepositoryPathFromReference(reference)
	if repository == "" {
		return ""
	}
	return path.Join("dmcr", "docker", "registry", "v2", "repositories", repository)
}

func RepositoryPathFromReference(reference string) string {
	if strings.Contains(reference, "://") {
		return ""
	}
	cleanReference := strings.TrimSpace(strings.SplitN(reference, "@", 2)[0])
	registry, repository, found := strings.Cut(cleanReference, "/")
	if !found || !looksLikeRegistry(registry) {
		return ""
	}
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	if repository == "" || strings.Contains(repository, "\\") {
		return ""
	}
	if !safeRepositoryPath(repository) {
		return ""
	}
	repositoryPart := repository[strings.LastIndex(repository, "/")+1:]
	if strings.Contains(repositoryPart, ":") {
		repository = repository[:strings.LastIndex(repository, ":")]
	}
	if repository == "" || !safeRepositoryPath(repository) {
		return ""
	}
	return repository
}

func looksLikeRegistry(value string) bool {
	value = strings.TrimSpace(value)
	return value == "localhost" || strings.Contains(value, ".") || strings.Contains(value, ":")
}

func safeRepositoryPath(value string) bool {
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
