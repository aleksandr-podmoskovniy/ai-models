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

func TestNormalizeDesiredArtifactsDeduplicatesAndSorts(t *testing.T) {
	t.Parallel()

	artifacts, err := NormalizeDesiredArtifacts([]DesiredArtifact{
		{ArtifactURI: "oci://example/model-b", Digest: "sha256:b"},
		{ArtifactURI: "oci://example/model-a", Digest: "sha256:a"},
		{ArtifactURI: "oci://example/model-a", Digest: "sha256:a", Family: "gguf-v1"},
	})
	if err != nil {
		t.Fatalf("NormalizeDesiredArtifacts() error = %v", err)
	}
	if got, want := len(artifacts), 2; got != want {
		t.Fatalf("artifact count = %d, want %d", got, want)
	}
	if got, want := artifacts[0].Digest, "sha256:a"; got != want {
		t.Fatalf("first digest = %q, want %q", got, want)
	}
	if got, want := artifacts[0].Family, "gguf-v1"; got != want {
		t.Fatalf("family = %q, want %q", got, want)
	}
}

func TestNormalizeDesiredArtifactsRejectsConflictingURI(t *testing.T) {
	t.Parallel()

	_, err := NormalizeDesiredArtifacts([]DesiredArtifact{
		{ArtifactURI: "oci://example/model-a", Digest: "sha256:a"},
		{ArtifactURI: "oci://example/model-b", Digest: "sha256:a"},
	})
	if err == nil || err.Error() != `node cache desired digest "sha256:a" maps to multiple artifact URIs` {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestDesiredArtifactsFromWorkloadAnnotationsReadsSingleArtifact(t *testing.T) {
	t.Parallel()

	artifacts, found, err := DesiredArtifactsFromWorkloadAnnotations(map[string]string{
		WorkloadResolvedDeliveryModeAnnotation:   WorkloadDeliveryModeSharedDirect,
		WorkloadResolvedDeliveryReasonAnnotation: WorkloadDeliveryReasonNodeCacheRuntime,
		WorkloadResolvedArtifactURIAnnotation:    "oci://example/model-a",
		WorkloadResolvedDigestAnnotation:         "sha256:a",
		WorkloadResolvedArtifactFamilyAnnotation: "gguf-v1",
	})
	if err != nil {
		t.Fatalf("DesiredArtifactsFromWorkloadAnnotations() error = %v", err)
	}
	if !found || len(artifacts) != 1 {
		t.Fatalf("unexpected artifacts %#v found=%v", artifacts, found)
	}
	if got, want := artifacts[0].Digest, "sha256:a"; got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
	if got, want := artifacts[0].Family, "gguf-v1"; got != want {
		t.Fatalf("family = %q, want %q", got, want)
	}
}

func TestDesiredArtifactsFromWorkloadAnnotationsReadsResolvedModels(t *testing.T) {
	t.Parallel()

	artifacts, found, err := DesiredArtifactsFromWorkloadAnnotations(map[string]string{
		WorkloadResolvedDeliveryModeAnnotation:   WorkloadDeliveryModeSharedDirect,
		WorkloadResolvedDeliveryReasonAnnotation: WorkloadDeliveryReasonNodeCacheRuntime,
		WorkloadResolvedModelsAnnotation: `[{"alias":"main","uri":"oci://example/model-a","digest":"sha256:a"},` +
			`{"alias":"draft","uri":"oci://example/model-b","digest":"sha256:b","family":"safetensors-v1"}]`,
	})
	if err != nil {
		t.Fatalf("DesiredArtifactsFromWorkloadAnnotations() error = %v", err)
	}
	if !found || len(artifacts) != 2 {
		t.Fatalf("unexpected artifacts %#v found=%v", artifacts, found)
	}
	if got, want := artifacts[1].Family, "safetensors-v1"; got != want {
		t.Fatalf("second family = %q, want %q", got, want)
	}
}

func TestDesiredArtifactsFromWorkloadAnnotationsIgnoresNonNodeCacheDelivery(t *testing.T) {
	t.Parallel()

	artifacts, found, err := DesiredArtifactsFromWorkloadAnnotations(map[string]string{
		WorkloadResolvedDeliveryModeAnnotation: "MaterializeBridge",
		WorkloadResolvedDigestAnnotation:       "sha256:a",
		WorkloadResolvedArtifactURIAnnotation:  "oci://example/model-a",
	})
	if err != nil {
		t.Fatalf("DesiredArtifactsFromWorkloadAnnotations() error = %v", err)
	}
	if found || len(artifacts) != 0 {
		t.Fatalf("unexpected artifacts %#v found=%v", artifacts, found)
	}
}
