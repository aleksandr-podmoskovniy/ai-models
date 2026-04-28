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

package nodecache

import "testing"

func TestValidateDesiredArtifactsFit(t *testing.T) {
	t.Parallel()

	err := ValidateDesiredArtifactsFit(100, []DesiredArtifact{
		{ArtifactURI: "oci://a", Digest: "sha256:a", SizeBytes: 40},
		{ArtifactURI: "oci://b", Digest: "sha256:b", SizeBytes: 50},
	})
	if err != nil {
		t.Fatalf("ValidateDesiredArtifactsFit() error = %v", err)
	}

	err = ValidateDesiredArtifactsFit(100, []DesiredArtifact{
		{ArtifactURI: "oci://a", Digest: "sha256:a", SizeBytes: 60},
		{ArtifactURI: "oci://b", Digest: "sha256:b", SizeBytes: 50},
	})
	if err == nil {
		t.Fatal("ValidateDesiredArtifactsFit() error = nil, want capacity failure")
	}
}

func TestValidateDesiredArtifactsFitFailsWithoutSizes(t *testing.T) {
	t.Parallel()

	err := ValidateDesiredArtifactsFit(100, []DesiredArtifact{
		{ArtifactURI: "oci://a", Digest: "sha256:a"},
	})
	if err == nil {
		t.Fatal("ValidateDesiredArtifactsFit() error = nil, want missing size failure")
	}
}
