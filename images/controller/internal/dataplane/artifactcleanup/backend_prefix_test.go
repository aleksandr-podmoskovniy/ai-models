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
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func TestBackendRepositoryMetadataPrefixFallsBackToReference(t *testing.T) {
	t.Parallel()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef",
		},
	}

	if got, want := backendRepositoryMetadataPrefix(handle), "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111"; got != want {
		t.Fatalf("unexpected metadata prefix %q", got)
	}
}

func TestBackendRepositoryMetadataPrefixPrefersStoredValue(t *testing.T) {
	t.Parallel()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference:                "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef",
			RepositoryMetadataPrefix: " /stored/repository/prefix/ ",
		},
	}

	if got, want := backendRepositoryMetadataPrefix(handle), "stored/repository/prefix"; got != want {
		t.Fatalf("unexpected metadata prefix %q", got)
	}
}

func TestBackendObjectStoragePrefixesIncludesSourceMirror(t *testing.T) {
	t.Parallel()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference:                "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef",
			SourceMirrorPrefix:       "raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef",
			RepositoryMetadataPrefix: "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111",
		},
	}

	prefixes := backendObjectStoragePrefixes(handle)
	if got, want := len(prefixes), 2; got != want {
		t.Fatalf("unexpected prefix count %d", got)
	}
	if got, want := prefixes[1], "raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef"; got != want {
		t.Fatalf("unexpected source mirror prefix %q", got)
	}
}
