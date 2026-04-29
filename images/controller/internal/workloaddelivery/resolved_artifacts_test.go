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

package workloaddelivery

import "testing"

func TestResolvedArtifactsFromAnnotationsReadsSingleArtifact(t *testing.T) {
	t.Parallel()

	artifacts, found, err := ResolvedArtifactsFromAnnotations(map[string]string{
		ResolvedDeliveryModeAnnotation:   DeliveryModeSharedDirect,
		ResolvedDeliveryReasonAnnotation: DeliveryReasonNodeSharedRuntimePlane,
		ResolvedArtifactURIAnnotation:    "oci://example/model-a",
		ResolvedDigestAnnotation:         "sha256:a",
		ResolvedArtifactFamilyAnnotation: "gguf-v1",
	})
	if err != nil {
		t.Fatalf("ResolvedArtifactsFromAnnotations() error = %v", err)
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

func TestResolvedArtifactsFromAnnotationsReadsResolvedModels(t *testing.T) {
	t.Parallel()

	artifacts, found, err := ResolvedArtifactsFromAnnotations(map[string]string{
		ResolvedDeliveryModeAnnotation:   DeliveryModeSharedDirect,
		ResolvedDeliveryReasonAnnotation: DeliveryReasonNodeSharedRuntimePlane,
		ResolvedModelsAnnotation: `[{"alias":"main","uri":"oci://example/model-a","digest":"sha256:a","sizeBytes":42},` +
			`{"alias":"draft","uri":"oci://example/model-b","digest":"sha256:b","family":"safetensors-v1","sizeBytes":84}]`,
	})
	if err != nil {
		t.Fatalf("ResolvedArtifactsFromAnnotations() error = %v", err)
	}
	if !found || len(artifacts) != 2 {
		t.Fatalf("unexpected artifacts %#v found=%v", artifacts, found)
	}
	if got, want := artifacts[1].Family, "safetensors-v1"; got != want {
		t.Fatalf("second family = %q, want %q", got, want)
	}
	if got, want := artifacts[1].SizeBytes, int64(84); got != want {
		t.Fatalf("second size bytes = %d, want %d", got, want)
	}
}

func TestResolvedArtifactsFromAnnotationsIgnoresNonSharedDirectDelivery(t *testing.T) {
	t.Parallel()

	artifacts, found, err := ResolvedArtifactsFromAnnotations(map[string]string{
		ResolvedDeliveryModeAnnotation: "LegacyBridge",
		ResolvedDigestAnnotation:       "sha256:a",
		ResolvedArtifactURIAnnotation:  "oci://example/model-a",
	})
	if err != nil {
		t.Fatalf("ResolvedArtifactsFromAnnotations() error = %v", err)
	}
	if found || len(artifacts) != 0 {
		t.Fatalf("unexpected artifacts %#v found=%v", artifacts, found)
	}
}
