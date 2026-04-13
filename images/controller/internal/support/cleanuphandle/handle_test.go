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

package cleanuphandle

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetOnObjectAndFromObjectRoundTrip(t *testing.T) {
	t.Parallel()

	object := &metav1.PartialObjectMetadata{}
	handle := Handle{
		Kind: KindBackendArtifact,
		Artifact: &ArtifactSnapshot{
			Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:    "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef",
			Digest: "sha256:deadbeef",
		},
		Backend: &BackendArtifactHandle{
			Reference:                "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef",
			RepositoryMetadataPrefix: "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/deepseek-r1",
			SourceMirrorPrefix:       "raw/1111-2222/source-url/.mirror/huggingface/deepseek-ai/DeepSeek-R1/deadbeef",
		},
	}

	if err := SetOnObject(object, handle); err != nil {
		t.Fatalf("SetOnObject() error = %v", err)
	}

	decoded, found, err := FromObject(object)
	if err != nil {
		t.Fatalf("FromObject() error = %v", err)
	}
	if !found {
		t.Fatal("expected cleanup handle annotation")
	}

	if got, want := decoded.Backend.Reference, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected backend reference %q", got)
	}
	if got, want := decoded.Backend.RepositoryMetadataPrefix, "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/deepseek-r1"; got != want {
		t.Fatalf("unexpected backend repository metadata prefix %q", got)
	}
	if got, want := decoded.Backend.SourceMirrorPrefix, "raw/1111-2222/source-url/.mirror/huggingface/deepseek-ai/DeepSeek-R1/deadbeef"; got != want {
		t.Fatalf("unexpected backend source mirror prefix %q", got)
	}
}

func TestDecodeRejectsIncompleteHandle(t *testing.T) {
	t.Parallel()

	if _, err := Decode(`{"kind":"BackendArtifact","backend":{}}`); err == nil {
		t.Fatal("expected error for incomplete backend cleanup handle")
	}
}

func TestDecodeAcceptsUploadStagingHandle(t *testing.T) {
	t.Parallel()

	handle, err := Decode(`{"kind":"UploadStaging","uploadStaging":{"bucket":"ai-models","key":"uploads/u1/payload.bin","fileName":"payload.bin","sizeBytes":123}}`)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if handle.UploadStaging == nil {
		t.Fatal("expected upload staging payload")
	}
	if got, want := handle.UploadStaging.Key, "uploads/u1/payload.bin"; got != want {
		t.Fatalf("unexpected staging key %q", got)
	}
}
