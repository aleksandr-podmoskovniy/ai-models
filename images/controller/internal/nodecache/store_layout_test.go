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
	"path/filepath"
	"testing"
)

func TestStorePathBuildsDigestAddressedLocation(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	if got, want := StorePath(cacheRoot, "sha256:deadbeef"), filepath.Join(cacheRoot, StoreDirName, "sha256:deadbeef"); got != want {
		t.Fatalf("store path = %q, want %q", got, want)
	}
}

func TestDigestFromArtifactURI(t *testing.T) {
	t.Parallel()

	if got, want := DigestFromArtifactURI("registry.local/catalog/model@sha256:deadbeef"), "sha256:deadbeef"; got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
	if got := DigestFromArtifactURI("registry.local/catalog/model:latest"); got != "" {
		t.Fatalf("expected empty digest for mutable reference, got %q", got)
	}
}
