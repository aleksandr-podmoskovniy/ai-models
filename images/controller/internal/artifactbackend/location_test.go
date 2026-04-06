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

package artifactbackend

import (
	"testing"

	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildOCIArtifactReferenceNamespaced(t *testing.T) {
	t.Parallel()

	ref, err := BuildOCIArtifactReference("registry.internal.local/ai-models", publication.Identity{
		Scope:     publication.ScopeNamespaced,
		Namespace: "team-a",
		Name:      "deepseek-r1",
	}, types.UID("550e8400-e29b-41d4-a716-446655440000"))
	if err != nil {
		t.Fatalf("BuildOCIArtifactReference() error = %v", err)
	}

	if got, want := ref, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000:published"; got != want {
		t.Fatalf("unexpected artifact reference %q", got)
	}
}

func TestBuildOCIArtifactReferenceCluster(t *testing.T) {
	t.Parallel()

	ref, err := BuildOCIArtifactReference("registry.internal.local/ai-models", publication.Identity{
		Scope: publication.ScopeCluster,
		Name:  "mixtral-8x7b",
	}, types.UID("1111-2222"))
	if err != nil {
		t.Fatalf("BuildOCIArtifactReference() error = %v", err)
	}

	if got, want := ref, "registry.internal.local/ai-models/catalog/cluster/mixtral-8x7b/1111-2222:published"; got != want {
		t.Fatalf("unexpected artifact reference %q", got)
	}
}

func TestBuildOCIArtifactReferenceRejectsSchemedRoot(t *testing.T) {
	t.Parallel()

	if _, err := BuildOCIArtifactReference("oci://registry.internal.local/ai-models", publication.Identity{
		Scope: publication.ScopeCluster,
		Name:  "mixtral-8x7b",
	}, types.UID("1111-2222")); err == nil {
		t.Fatal("expected error for schemed OCI root")
	}
}
