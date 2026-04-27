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

package publishobserve

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"k8s.io/apimachinery/pkg/types"
)

func testRequest() publicationports.Request {
	return publicationports.Request{
		Owner: publicationports.Owner{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      "deepseek-r1",
			Namespace: "team-a",
			UID:       types.UID("550e8400-e29b-41d4-a716-446655440000"),
		},
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: "team-a",
			Name:      "deepseek-r1",
		},
		Spec: modelsv1alpha1.ModelSpec{
			Source: modelsv1alpha1.ModelSourceSpec{
				URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
			},
		},
	}
}

func succeededTerminationMessage(t *testing.T) string {
	t.Helper()

	payload, err := publicationartifact.EncodeResult(publicationartifact.Result{
		Artifact: publication.PublishedArtifact{
			Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:       "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			Digest:    "sha256:deadbeef",
			MediaType: "application/vnd.cncf.model.manifest.v1+json",
			SizeBytes: 42,
		},
		Resolved: publication.ResolvedProfile{
			Task:                   "text-generation",
			TaskConfidence:         publication.ProfileConfidenceDerived,
			Family:                 "deepseek",
			FamilyConfidence:       publication.ProfileConfidenceExact,
			License:                "apache-2.0",
			Architecture:           "DeepseekForCausalLM",
			ArchitectureConfidence: publication.ProfileConfidenceExact,
			Format:                 "Safetensors",
			SourceRepoID:           "deepseek-ai/DeepSeek-R1",
		},
		Source: publication.SourceProvenance{
			Type:              modelsv1alpha1.ModelSourceTypeHuggingFace,
			ExternalReference: "deepseek-ai/DeepSeek-R1",
			ResolvedRevision:  "abc123",
		},
		CleanupHandle: cleanuphandle.Handle{
			Kind: cleanuphandle.KindBackendArtifact,
			Artifact: &cleanuphandle.ArtifactSnapshot{
				Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:  "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			},
			Backend: &cleanuphandle.BackendArtifactHandle{
				Reference: "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			},
		},
	})
	if err != nil {
		t.Fatalf("EncodeResult() error = %v", err)
	}
	return payload
}

func uploadStagingTerminationMessage(t *testing.T) string {
	t.Helper()

	payload, err := cleanuphandle.Encode(cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "ai-models",
			Key:       "raw/550e8400-e29b-41d4-a716-446655440000/model.gguf",
			FileName:  "model.gguf",
			SizeBytes: 42,
		},
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	return payload
}
