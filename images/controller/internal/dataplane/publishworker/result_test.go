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

package publishworker

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestAttachResolvedProfileProvenance(t *testing.T) {
	t.Parallel()

	resolved := attachResolvedProfileProvenance(publicationdata.ResolvedProfile{
		Task:   "text-generation",
		Format: "Safetensors",
	}, sourceProfileProvenance{
		License:      "apache-2.0",
		SourceRepoID: "deepseek-ai/DeepSeek-R1",
	})

	if got, want := resolved.License, "apache-2.0"; got != want {
		t.Fatalf("unexpected license %q", got)
	}
	if got, want := resolved.SourceRepoID, "deepseek-ai/DeepSeek-R1"; got != want {
		t.Fatalf("unexpected source repo ID %q", got)
	}
}

func TestBuildBackendResultSetsRepositoryMetadataPrefix(t *testing.T) {
	t.Parallel()

	result := buildBackendResult(
		publicationdata.SourceProvenance{Type: modelsv1alpha1.ModelSourceTypeHuggingFace},
		publicationdata.ResolvedProfile{Task: "text-generation", Format: "Safetensors"},
		modelpackports.PublishResult{
			Reference: "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef",
			Digest:    "sha256:deadbeef",
			MediaType: "application/vnd.cncf.model.manifest.v1+json",
			SizeBytes: 123,
		},
	)

	if result.CleanupHandle.Backend == nil {
		t.Fatal("expected backend cleanup handle")
	}
	if got, want := result.CleanupHandle.Backend.RepositoryMetadataPrefix, "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111"; got != want {
		t.Fatalf("unexpected repository metadata prefix %q", got)
	}
}

func TestAttachBackendSourceMirror(t *testing.T) {
	t.Parallel()

	result := buildBackendResult(
		publicationdata.SourceProvenance{Type: modelsv1alpha1.ModelSourceTypeHuggingFace},
		publicationdata.ResolvedProfile{Task: "text-generation", Format: "Safetensors"},
		modelpackports.PublishResult{
			Reference: "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef",
			Digest:    "sha256:deadbeef",
			MediaType: "application/vnd.cncf.model.manifest.v1+json",
			SizeBytes: 123,
		},
	)

	result = attachBackendSourceMirror(result, &sourcefetch.SourceMirrorSnapshot{
		CleanupPrefix: "raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef",
	})

	if result.CleanupHandle.Backend == nil {
		t.Fatal("expected backend cleanup handle")
	}
	if got, want := result.CleanupHandle.Backend.SourceMirrorPrefix, "raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef"; got != want {
		t.Fatalf("unexpected source mirror prefix %q", got)
	}
}
