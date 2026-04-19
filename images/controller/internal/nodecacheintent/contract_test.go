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

package nodecacheintent

import "testing"

func TestNormalizeIntentsDeduplicatesAndSorts(t *testing.T) {
	t.Parallel()

	intents, err := NormalizeIntents([]ArtifactIntent{
		{ArtifactURI: "oci://example/model-b", Digest: "sha256:b"},
		{ArtifactURI: "oci://example/model-a", Digest: "sha256:a"},
		{ArtifactURI: "oci://example/model-a", Digest: "sha256:a", Family: "gguf-v1"},
	})
	if err != nil {
		t.Fatalf("NormalizeIntents() error = %v", err)
	}
	if got, want := len(intents), 2; got != want {
		t.Fatalf("intent count = %d, want %d", got, want)
	}
	if got, want := intents[0].Digest, "sha256:a"; got != want {
		t.Fatalf("first digest = %q, want %q", got, want)
	}
	if got, want := intents[0].Family, "gguf-v1"; got != want {
		t.Fatalf("family = %q, want %q", got, want)
	}
}

func TestNormalizeIntentsRejectsConflictingURI(t *testing.T) {
	t.Parallel()

	_, err := NormalizeIntents([]ArtifactIntent{
		{ArtifactURI: "oci://example/model-a", Digest: "sha256:a"},
		{ArtifactURI: "oci://example/model-b", Digest: "sha256:a"},
	})
	if err == nil || err.Error() != `node cache intent digest "sha256:a" maps to multiple artifact URIs` {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	payload, err := EncodeIntents([]ArtifactIntent{{
		ArtifactURI: "oci://example/model-a",
		Digest:      "sha256:a",
		Family:      "hf-safetensors-v1",
	}})
	if err != nil {
		t.Fatalf("EncodeIntents() error = %v", err)
	}
	intents, err := DecodeIntents(payload)
	if err != nil {
		t.Fatalf("DecodeIntents() error = %v", err)
	}
	if got, want := len(intents), 1; got != want {
		t.Fatalf("decoded count = %d, want %d", got, want)
	}
	if got, want := intents[0].ArtifactURI, "oci://example/model-a"; got != want {
		t.Fatalf("artifact URI = %q, want %q", got, want)
	}
}
