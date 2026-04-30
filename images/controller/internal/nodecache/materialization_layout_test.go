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
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMaterializationLayoutFromReferenceDigest(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	layout, err := ResolveMaterializationLayout(cacheRoot, "registry.local/catalog/model@sha256:deadbeef", "")
	if err != nil {
		t.Fatalf("ResolveMaterializationLayout() error = %v", err)
	}
	if got, want := layout.ArtifactDigest, "sha256:deadbeef"; got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
	if got, want := layout.DestinationDir, filepath.Join(cacheRoot, StoreDirName, layout.ArtifactDigest); got != want {
		t.Fatalf("destination = %q, want %q", got, want)
	}
	if got, want := layout.CurrentLinkPath, filepath.Join(cacheRoot, CurrentLinkName); got != want {
		t.Fatalf("current link path = %q, want %q", got, want)
	}
}

func TestUpdateCurrentLinkCreatesRelativeSymlink(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	targetPath := filepath.Join(cacheRoot, StoreDirName, "sha256:deadbeef", "model")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetPath) error = %v", err)
	}
	if err := UpdateCurrentLink(cacheRoot, targetPath); err != nil {
		t.Fatalf("UpdateCurrentLink() error = %v", err)
	}
	linkTarget, err := os.Readlink(CurrentLinkPath(cacheRoot))
	if err != nil {
		t.Fatalf("Readlink(current) error = %v", err)
	}
	if got, want := linkTarget, filepath.Join(StoreDirName, "sha256:deadbeef", "model"); got != want {
		t.Fatalf("symlink target = %q, want %q", got, want)
	}
}

func TestUpdateWorkloadModelLinkTargetsInternalCurrentLink(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	targetPath := filepath.Join(cacheRoot, StoreDirName, "sha256:deadbeef", "model")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetPath) error = %v", err)
	}
	if err := UpdateCurrentLink(cacheRoot, targetPath); err != nil {
		t.Fatalf("UpdateCurrentLink() error = %v", err)
	}
	if err := UpdateWorkloadModelLink(cacheRoot); err != nil {
		t.Fatalf("UpdateWorkloadModelLink() error = %v", err)
	}

	linkTarget, err := os.Readlink(WorkloadModelPath(cacheRoot))
	if err != nil {
		t.Fatalf("Readlink(model) error = %v", err)
	}
	if got, want := linkTarget, CurrentLinkName; got != want {
		t.Fatalf("workload model target = %q, want %q", got, want)
	}
}

func TestSharedArtifactModelPathUsesDigestStoreContractPath(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	if got, want := SharedArtifactModelPath(cacheRoot, "sha256:deadbeef"), filepath.Join(cacheRoot, StoreDirName, "sha256:deadbeef", "model"); got != want {
		t.Fatalf("shared artifact model path = %q, want %q", got, want)
	}
}

func TestWorkloadNamedModelPathUsesStableModelsDirectory(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	if got, want := WorkloadNamedModelPath(cacheRoot, "main"), filepath.Join(cacheRoot, "models", "main"); got != want {
		t.Fatalf("workload named model path = %q, want %q", got, want)
	}
}

func TestUpdateWorkloadNamedModelLinkCreatesRelativeSymlink(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	targetPath := filepath.Join(cacheRoot, StoreDirName, "sha256:deadbeef", "model")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetPath) error = %v", err)
	}
	if err := UpdateWorkloadNamedModelLink(cacheRoot, "main", targetPath); err != nil {
		t.Fatalf("UpdateWorkloadNamedModelLink() error = %v", err)
	}
	linkTarget, err := os.Readlink(WorkloadNamedModelPath(cacheRoot, "main"))
	if err != nil {
		t.Fatalf("Readlink(named model) error = %v", err)
	}
	if filepath.IsAbs(linkTarget) {
		t.Fatalf("expected relative named-model symlink, got %q", linkTarget)
	}
}

func TestValidateWorkloadModelNameRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "Main", "main_model", "-main", "main-"} {
		if err := ValidateWorkloadModelName(name); err == nil {
			t.Fatalf("expected model name %q to be rejected", name)
		}
	}
	if err := ValidateWorkloadModelName("draft-model2.v1"); err != nil {
		t.Fatalf("expected valid model name: %v", err)
	}
}
