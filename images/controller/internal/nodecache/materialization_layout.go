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

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"k8s.io/apimachinery/pkg/util/validation"
)

type MaterializationLayout struct {
	DestinationDir  string
	CurrentLinkPath string
	ArtifactDigest  string
}

func CurrentLinkPath(cacheRoot string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(cacheRoot)), CurrentLinkName)
}

func WorkloadModelPath(cacheRoot string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(cacheRoot)), WorkloadLinkName)
}

func WorkloadModelsDirPath(cacheRoot string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(cacheRoot)), "models")
}

func WorkloadNamedModelPath(cacheRoot, name string) string {
	return filepath.Join(WorkloadModelsDirPath(cacheRoot), strings.TrimSpace(name))
}

func SharedArtifactModelPath(cacheRoot, digest string) string {
	return modelpackports.MaterializedModelPath(StorePath(cacheRoot, digest))
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
	return updateRelativeLink(CurrentLinkPath(cacheRoot), targetModelPath)
}

func UpdateWorkloadModelLink(cacheRoot string) error {
	cacheRoot = filepath.Clean(strings.TrimSpace(cacheRoot))
	if cacheRoot == "" || cacheRoot == "." {
		return errors.New("cache-root workload model symlink requires non-empty path")
	}
	return updateRelativeLink(WorkloadModelPath(cacheRoot), CurrentLinkPath(cacheRoot))
}

func UpdateWorkloadNamedModelLink(cacheRoot, name, targetModelPath string) error {
	cacheRoot = filepath.Clean(strings.TrimSpace(cacheRoot))
	name = strings.TrimSpace(name)
	if err := ValidateWorkloadModelName(name); err != nil {
		return err
	}
	if cacheRoot == "" || cacheRoot == "." {
		return errors.New("cache-root workload named model symlink requires non-empty path")
	}
	return updateRelativeLink(WorkloadNamedModelPath(cacheRoot, name), targetModelPath)
}

func ValidateWorkloadModelName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("workload model name must not be empty")
	}
	if problems := validation.IsDNS1123Subdomain(name); len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func updateRelativeLink(linkPath, targetPath string) error {
	linkPath = filepath.Clean(strings.TrimSpace(linkPath))
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if linkPath == "" || linkPath == "." || targetPath == "" || targetPath == "." {
		return errors.New("relative symlink update requires non-empty paths")
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}

	relativeTarget, err := filepath.Rel(filepath.Dir(linkPath), targetPath)
	if err != nil {
		return err
	}
	tempLink := linkPath + ".tmp"
	_ = os.Remove(tempLink)
	if err := os.Symlink(relativeTarget, tempLink); err != nil {
		return err
	}
	if err := os.Rename(tempLink, linkPath); err != nil {
		_ = os.Remove(tempLink)
		return err
	}
	return nil
}
