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

package publicationartifact

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func TestResultValidateRequiresArtifactAndCleanupHandle(t *testing.T) {
	t.Parallel()

	result := Result{
		Source: publication.SourceProvenance{
			Type: modelsv1alpha1.ModelSourceTypeHTTP,
		},
		Artifact: publication.PublishedArtifact{
			Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:  "backend.example/catalog/team-a/deepseek-r1@sha256:deadbeef",
		},
		CleanupHandle: cleanuphandle.Handle{
			Kind: cleanuphandle.KindBackendArtifact,
			Artifact: &cleanuphandle.ArtifactSnapshot{
				Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:  "backend.example/catalog/team-a/deepseek-r1@sha256:deadbeef",
			},
			Backend: &cleanuphandle.BackendArtifactHandle{
				Reference: "backend.example/catalog/team-a/deepseek-r1@sha256:deadbeef",
			},
		},
	}

	if err := result.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDecodeResultRoundTrip(t *testing.T) {
	t.Parallel()

	raw := `{"source":{"type":"HuggingFace","externalReference":"deepseek-ai/DeepSeek-R1","resolvedRevision":"abc123"},"artifact":{"kind":"OCI","uri":"registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef","digest":"sha256:deadbeef","mediaType":"application/vnd.cncf.model.manifest.v1+json","sizeBytes":123},"resolved":{"task":"text-generation","framework":"transformers","family":"deepseek","architecture":"DeepseekForCausalLM","format":"Safetensors","contextWindowTokens":8192,"sourceRepoID":"deepseek-ai/DeepSeek-R1"},"cleanupHandle":{"kind":"BackendArtifact","artifact":{"kind":"OCI","uri":"registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef","digest":"sha256:deadbeef"},"backend":{"reference":"registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef"}}}`

	result, err := DecodeResult(raw)
	if err != nil {
		t.Fatalf("DecodeResult() error = %v", err)
	}

	if got, want := result.Artifact.URI, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected artifact URI %q", got)
	}
}
