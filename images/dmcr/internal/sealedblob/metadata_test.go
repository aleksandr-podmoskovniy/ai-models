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

package sealedblob

import "testing"

func TestMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	payload, err := Marshal(Metadata{
		Version:      MetadataVersion,
		Digest:       "sha256:test",
		PhysicalPath: "dmcr/_ai_models/direct-upload/objects/session/data",
		SizeBytes:    123,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	got, err := Unmarshal(payload)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.PhysicalPath != "dmcr/_ai_models/direct-upload/objects/session/data" {
		t.Fatalf("PhysicalPath = %q, want %q", got.PhysicalPath, "dmcr/_ai_models/direct-upload/objects/session/data")
	}
}

func TestMetadataPathHelpers(t *testing.T) {
	t.Parallel()

	const canonicalPath = "/docker/registry/v2/blobs/sha256/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/data"

	got := MetadataPath(canonicalPath)
	if got != canonicalPath+MetadataSuffix {
		t.Fatalf("MetadataPath() = %q, want %q", got, canonicalPath+MetadataSuffix)
	}
	if !IsMetadataPath(got) {
		t.Fatal("IsMetadataPath() = false, want true")
	}
	if !LooksLikeCanonicalBlobDataPath(canonicalPath) {
		t.Fatal("LooksLikeCanonicalBlobDataPath() = false, want true")
	}

	restored, ok := CanonicalPathFromMetadataPath(got)
	if !ok {
		t.Fatal("CanonicalPathFromMetadataPath() ok = false, want true")
	}
	if restored != canonicalPath {
		t.Fatalf("CanonicalPathFromMetadataPath() = %q, want %q", restored, canonicalPath)
	}

	resolvedDigest, ok := DigestFromCanonicalBlobDataPath(canonicalPath)
	if !ok {
		t.Fatal("DigestFromCanonicalBlobDataPath() ok = false, want true")
	}
	if resolvedDigest != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("DigestFromCanonicalBlobDataPath() = %q, want %q", resolvedDigest, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	}
}
