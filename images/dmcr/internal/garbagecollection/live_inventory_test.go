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

package garbagecollection

import "testing"

func TestCollectLivePrefixesFromObjectUsesExplicitPrefixes(t *testing.T) {
	t.Parallel()

	live := newLivePrefixSet()
	annotations := map[string]string{
		cleanupHandleAnnotationKey: `{"kind":"BackendArtifact","backend":{"repositoryMetadataPrefix":"dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111","sourceMirrorPrefix":"raw/1111/source-url/.mirror/huggingface/owner/model/deadbeef"}}`,
	}

	if err := collectLivePrefixesFromObject("team-a", "model-a", annotations, &live); err != nil {
		t.Fatalf("collectLivePrefixesFromObject() error = %v", err)
	}
	if _, found := live.repositoryPrefixes["dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111"]; !found {
		t.Fatal("expected repository prefix to be collected")
	}
	if _, found := live.rawPrefixes["raw/1111/source-url/.mirror/huggingface/owner/model/deadbeef"]; !found {
		t.Fatal("expected raw source mirror prefix to be collected")
	}
}

func TestCollectLivePrefixesFromObjectFallsBackToReference(t *testing.T) {
	t.Parallel()

	live := newLivePrefixSet()
	annotations := map[string]string{
		cleanupHandleAnnotationKey: `{"kind":"BackendArtifact","backend":{"reference":"dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/cluster/gemma/2222@sha256:deadbeef"}}`,
	}

	if err := collectLivePrefixesFromObject("", "gemma", annotations, &live); err != nil {
		t.Fatalf("collectLivePrefixesFromObject() error = %v", err)
	}
	if _, found := live.repositoryPrefixes["dmcr/docker/registry/v2/repositories/ai-models/catalog/cluster/gemma/2222"]; !found {
		t.Fatal("expected fallback repository prefix to be collected")
	}
}
