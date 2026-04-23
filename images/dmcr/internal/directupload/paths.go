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

package directupload

import (
	"fmt"
	"path"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

func BlobDataObjectKey(rootDirectory string, dgst string) (string, error) {
	parsedDigest, err := digest.Parse(strings.TrimSpace(dgst))
	if err != nil {
		return "", err
	}
	hexPart := parsedDigest.Encoded()
	return withRootDirectory(
		rootDirectory,
		path.Join(
			"docker/registry/v2/blobs",
			parsedDigest.Algorithm().String(),
			hexPart[:2],
			hexPart,
			"data",
		),
	), nil
}

func UploadSessionObjectKey(rootDirectory, sessionID string) (string, error) {
	cleanSessionID := strings.TrimSpace(sessionID)
	if cleanSessionID == "" {
		return "", fmt.Errorf("session ID must not be empty")
	}
	return withRootDirectory(
		rootDirectory,
		path.Join(
			"_ai_models",
			"direct-upload",
			"objects",
			cleanSessionID,
			"data",
		),
	), nil
}

func RepositoryBlobLinkObjectKey(rootDirectory, repository, dgst string) (string, error) {
	cleanRepository := strings.Trim(strings.TrimSpace(repository), "/")
	if cleanRepository == "" {
		return "", fmt.Errorf("repository must not be empty")
	}
	parsedDigest, err := digest.Parse(strings.TrimSpace(dgst))
	if err != nil {
		return "", err
	}
	return withRootDirectory(
		rootDirectory,
		path.Join(
			"docker/registry/v2/repositories",
			cleanRepository,
			"_layers",
			parsedDigest.Algorithm().String(),
			parsedDigest.Encoded(),
			"link",
		),
	), nil
}

func storageDriverPathForObjectKey(rootDirectory, objectKey string) string {
	cleanPath := strings.Trim(strings.TrimSpace(objectKey), "/")
	cleanRoot := strings.Trim(strings.TrimSpace(rootDirectory), "/")
	if cleanRoot != "" {
		cleanPath = strings.TrimPrefix(cleanPath, cleanRoot+"/")
	}
	if cleanPath == "" {
		return "/"
	}
	return "/" + cleanPath
}

func withRootDirectory(rootDirectory, objectPath string) string {
	cleanRoot := strings.Trim(strings.TrimSpace(rootDirectory), "/")
	cleanPath := strings.Trim(strings.TrimSpace(objectPath), "/")
	if cleanRoot == "" {
		return cleanPath
	}
	return path.Join(cleanRoot, cleanPath)
}
