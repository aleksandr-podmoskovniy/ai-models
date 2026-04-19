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

package nodecache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type MaterializationLayout struct {
	DestinationDir  string
	CurrentLinkPath string
	ArtifactDigest  string
}

func CurrentLinkPath(cacheRoot string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(cacheRoot)), CurrentLinkName)
}

func ResolveMaterializationLayout(cacheRoot, artifactURI, artifactDigest string) (MaterializationLayout, error) {
	cacheRoot = filepath.Clean(strings.TrimSpace(cacheRoot))
	if cacheRoot == "" || cacheRoot == "." {
		return MaterializationLayout{}, errors.New("cache-root must not be empty")
	}
	digest := strings.TrimSpace(artifactDigest)
	if digest == "" {
		digest = DigestFromArtifactURI(artifactURI)
	}
	if digest == "" {
		return MaterializationLayout{}, errors.New("cache-root requires immutable artifact digest")
	}
	return MaterializationLayout{
		DestinationDir:  StorePath(cacheRoot, digest),
		CurrentLinkPath: CurrentLinkPath(cacheRoot),
		ArtifactDigest:  digest,
	}, nil
}

func UpdateCurrentLink(cacheRoot, targetModelPath string) error {
	cacheRoot = filepath.Clean(strings.TrimSpace(cacheRoot))
	targetModelPath = filepath.Clean(strings.TrimSpace(targetModelPath))
	if cacheRoot == "" || cacheRoot == "." || targetModelPath == "" || targetModelPath == "." {
		return errors.New("cache-root current symlink requires non-empty paths")
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return err
	}

	currentPath := CurrentLinkPath(cacheRoot)
	relativeTarget, err := filepath.Rel(cacheRoot, targetModelPath)
	if err != nil {
		return err
	}
	tempLink := currentPath + ".tmp"
	_ = os.Remove(tempLink)
	if err := os.Symlink(relativeTarget, tempLink); err != nil {
		return err
	}
	if err := os.Rename(tempLink, currentPath); err != nil {
		_ = os.Remove(tempLink)
		return err
	}
	return nil
}
