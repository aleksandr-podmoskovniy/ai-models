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

package publication

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestSnapshotValidateAcceptsNamespacedPublication(t *testing.T) {
	t.Parallel()

	snapshot := Snapshot{
		Identity: Identity{
			Scope:     ScopeNamespaced,
			Namespace: "team-a",
			Name:      "deepseek-r1",
		},
		Source: SourceProvenance{
			Type: modelsv1alpha1.ModelSourceTypeHuggingFace,
		},
		Artifact: PublishedArtifact{
			Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:    "registry.example/ai-models/team-a/deepseek-r1@sha256:deadbeef",
			Digest: "sha256:deadbeef",
		},
		Result: Result{
			State: "Published",
			Ready: true,
		},
	}

	if err := snapshot.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestSnapshotValidateRejectsBrokenIdentity(t *testing.T) {
	t.Parallel()

	snapshot := Snapshot{
		Identity: Identity{
			Scope: ScopeNamespaced,
			Name:  "deepseek-r1",
		},
		Source: SourceProvenance{
			Type: modelsv1alpha1.ModelSourceTypeUpload,
		},
		Artifact: PublishedArtifact{
			Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:    "registry.example/ai-models/team-a/deepseek-r1@sha256:deadbeef",
			Digest: "sha256:deadbeef",
		},
		Result: Result{
			State: "Published",
		},
	}

	if err := snapshot.Validate(); err == nil {
		t.Fatal("expected error for namespaced publication without namespace")
	}
}
