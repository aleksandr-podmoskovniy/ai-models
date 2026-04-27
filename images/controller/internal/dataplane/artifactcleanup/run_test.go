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
	"context"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type fakeRemover struct {
	reference string
}

func (f *fakeRemover) Remove(_ context.Context, reference string, _ modelpackports.RegistryAuth) error {
	f.reference = reference
	return nil
}

type fakePrefixRemover struct {
	calls []uploadstagingports.DeletePrefixInput
}

func (f *fakePrefixRemover) DeletePrefix(_ context.Context, input uploadstagingports.DeletePrefixInput) error {
	f.calls = append(f.calls, input)
	return nil
}

func TestRunInvokesRemover(t *testing.T) {
	t.Parallel()

	remover := &fakeRemover{}
	err := Run(context.Background(), Options{
		HandleJSON: `{"kind":"BackendArtifact","backend":{"reference":"registry.example.com/model@sha256:deadbeef"}}`,
		Remover:    remover,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := remover.reference, "registry.example.com/model@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected reference %q", got)
	}
}

func TestRunPrunesBackendRepositoryMetadataPrefix(t *testing.T) {
	t.Parallel()

	remover := &fakeRemover{}
	prefixRemover := &fakePrefixRemover{}
	err := Run(context.Background(), Options{
		HandleJSON:          `{"kind":"BackendArtifact","artifact":{"kind":"OCI","uri":"registry.example.com/model@sha256:deadbeef"},"backend":{"reference":"registry.example.com/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef","repositoryMetadataPrefix":"dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111"}}`,
		Remover:             remover,
		PrefixRemover:       prefixRemover,
		ObjectStorageBucket: "artifacts",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := len(prefixRemover.calls), 1; got != want {
		t.Fatalf("unexpected prefix remover call count %d", got)
	}
	if got, want := prefixRemover.calls[0].Bucket, "artifacts"; got != want {
		t.Fatalf("unexpected prefix bucket %q", got)
	}
	if got, want := prefixRemover.calls[0].Prefix, "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111"; got != want {
		t.Fatalf("unexpected metadata prefix %q", got)
	}
}

func TestRunPrunesDerivedBackendRepositoryMetadataPrefix(t *testing.T) {
	t.Parallel()

	remover := &fakeRemover{}
	prefixRemover := &fakePrefixRemover{}
	err := Run(context.Background(), Options{
		HandleJSON:          `{"kind":"BackendArtifact","artifact":{"kind":"OCI","uri":"registry.example.com/model@sha256:deadbeef"},"backend":{"reference":"registry.example.com/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef"}}`,
		Remover:             remover,
		PrefixRemover:       prefixRemover,
		ObjectStorageBucket: "artifacts",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := len(prefixRemover.calls), 1; got != want {
		t.Fatalf("unexpected prefix remover call count %d", got)
	}
	if got, want := prefixRemover.calls[0].Prefix, "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111"; got != want {
		t.Fatalf("unexpected derived metadata prefix %q", got)
	}
}

func TestRunPrunesSourceMirrorPrefix(t *testing.T) {
	t.Parallel()

	remover := &fakeRemover{}
	prefixRemover := &fakePrefixRemover{}
	err := Run(context.Background(), Options{
		HandleJSON:          `{"kind":"BackendArtifact","artifact":{"kind":"OCI","uri":"registry.example.com/model@sha256:deadbeef"},"backend":{"reference":"registry.example.com/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef","repositoryMetadataPrefix":"dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111","sourceMirrorPrefix":"raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef"}}`,
		Remover:             remover,
		PrefixRemover:       prefixRemover,
		ObjectStorageBucket: "artifacts",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := len(prefixRemover.calls), 2; got != want {
		t.Fatalf("unexpected prefix remover call count %d", got)
	}
	if got, want := prefixRemover.calls[1].Prefix, "raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef"; got != want {
		t.Fatalf("unexpected source mirror prefix %q", got)
	}
}
