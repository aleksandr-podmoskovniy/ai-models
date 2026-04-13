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

package oci

import (
	"context"
	"strings"
	"testing"
)

func TestRegistryManifestURL(t *testing.T) {
	t.Parallel()

	got, err := RegistryManifestURL("registry.example.com/ai-models/catalog/model:published")
	if err != nil {
		t.Fatalf("RegistryManifestURL() error = %v", err)
	}
	want := "https://registry.example.com/v2/ai-models/catalog/model/manifests/published"
	if got != want {
		t.Fatalf("unexpected manifest URL %q", got)
	}
}

func TestRegistryManifestURLDigestReference(t *testing.T) {
	t.Parallel()

	got, err := RegistryManifestURL("registry.example.com/ai-models/catalog/model@sha256:deadbeef")
	if err != nil {
		t.Fatalf("RegistryManifestURL() error = %v", err)
	}
	want := "https://registry.example.com/v2/ai-models/catalog/model/manifests/sha256:deadbeef"
	if got != want {
		t.Fatalf("unexpected manifest URL %q", got)
	}
}

func TestInspectRemote(t *testing.T) {
	t.Parallel()

	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		layerTar: tarBytes(t, map[string]string{
			"model/config.json": "{}",
		}),
	})
	defer server.Close()

	payload, err := InspectRemote(context.Background(), serverReference(server, "published"), auth)
	if err != nil {
		t.Fatalf("InspectRemote() error = %v", err)
	}
	if err := ValidatePayload(payload); err != nil {
		t.Fatalf("ValidatePayload() error = %v", err)
	}
	if got, want := ArtifactDigest(payload), "sha256:deadbeef"; got != want {
		t.Fatalf("unexpected digest %q", got)
	}
	if got, want := ArtifactMediaType(payload), ModelPackArtifactType; got != want {
		t.Fatalf("unexpected mediaType %q", got)
	}
	if got, want := InspectSizeBytes(payload), int64(113); got != want {
		t.Fatalf("unexpected size %d", got)
	}
}

func TestValidatePayloadRejectsBrokenManifest(t *testing.T) {
	t.Parallel()

	err := ValidatePayload(InspectPayload{
		"digest": "sha256:deadbeef",
		"manifest": map[string]any{
			"schemaVersion": float64(2),
			"artifactType":  "application/vnd.oci.image.manifest.v1+json",
			"config": map[string]any{
				"mediaType": ModelPackConfigMediaType,
				"digest":    "sha256:config",
				"size":      float64(10),
			},
			"layers": []any{
				map[string]any{
					"mediaType": ModelPackWeightLayerType,
					"digest":    "sha256:layer",
					"size":      float64(30),
					"annotations": map[string]any{
						ModelPackFilepathAnnotation: "model",
					},
				},
			},
		},
		"configBlob": map[string]any{
			"descriptor": map[string]any{"name": "model"},
			"modelfs": map[string]any{
				"type":    "layers",
				"diffIds": []any{"sha256:layer-diff"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "artifactType") {
		t.Fatalf("expected artifactType validation error, got %v", err)
	}
}
