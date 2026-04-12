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

package kitops

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestRegistryFromOCIReference(t *testing.T) {
	t.Parallel()

	if got, want := registryFromOCIReference("registry.example.com/ai-models/catalog/model:published"), "registry.example.com"; got != want {
		t.Fatalf("unexpected registry %q", got)
	}
}

func TestImmutableOCIReference(t *testing.T) {
	t.Parallel()

	if got, want := immutableOCIReference("registry.example.com/ai-models/catalog/model:published", "sha256:deadbeef"), "registry.example.com/ai-models/catalog/model@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected immutable reference %q", got)
	}
}

func TestInspectModelPackSize(t *testing.T) {
	t.Parallel()

	size := inspectModelPackSize(map[string]any{
		"manifest": map[string]any{
			"config": map[string]any{"size": float64(10)},
			"layers": []any{
				map[string]any{"size": float64(30)},
				map[string]any{"size": float64(40)},
			},
		},
	})

	if got, want := size, int64(80); got != want {
		t.Fatalf("unexpected size %d", got)
	}
}

func TestRegistryManifestURL(t *testing.T) {
	t.Parallel()

	got, err := registryManifestURL("registry.example.com/ai-models/catalog/model:published")
	if err != nil {
		t.Fatalf("registryManifestURL() error = %v", err)
	}
	want := "https://registry.example.com/v2/ai-models/catalog/model/manifests/published"
	if got != want {
		t.Fatalf("unexpected manifest URL %q", got)
	}
}

func TestInspectRemoteViaRegistry(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/ai-models/catalog/model/manifests/published":
			if got := r.Header.Get("Accept"); !strings.Contains(got, "application/vnd.oci.image.manifest.v1+json") {
				t.Fatalf("unexpected Accept header %q", got)
			}
			user, pass, ok := r.BasicAuth()
			if !ok || user != "writer" || pass != "secret" {
				t.Fatalf("unexpected auth %q/%q", user, pass)
			}
			w.Header().Set("Docker-Content-Digest", "sha256:deadbeef")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"schemaVersion":2,"artifactType":"application/vnd.cncf.model.manifest.v1+json","config":{"mediaType":"application/vnd.cncf.model.config.v1+json","digest":"sha256:config","size":10},"layers":[{"mediaType":"application/vnd.cncf.model.weight.v1.tar","digest":"sha256:layer","size":30,"annotations":{"org.cncf.model.filepath":"model"}}]}`))
		case "/v2/ai-models/catalog/model/blobs/sha256:config":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "writer" || pass != "secret" {
				t.Fatalf("unexpected auth %q/%q", user, pass)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"descriptor":{"name":"model"},"modelfs":{"type":"layers","diffIds":["sha256:layer-diff"]},"config":{}}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	caFile := writeTempFile(t, certPEM)

	payload, err := inspectRemoteViaRegistry(context.Background(), strings.TrimPrefix(server.URL, "https://")+"/ai-models/catalog/model:published", modelpackports.RegistryAuth{
		Username: "writer",
		Password: "secret",
		CAFile:   caFile,
	})
	if err != nil {
		t.Fatalf("inspectRemoteViaRegistry() error = %v", err)
	}
	if err := validateModelPackPayload(payload); err != nil {
		t.Fatalf("validateModelPackPayload() error = %v", err)
	}

	if got, want := artifactDigestFromInspectPayload(payload), "sha256:deadbeef"; got != want {
		t.Fatalf("unexpected digest %q", got)
	}
	if got, want := artifactMediaTypeFromInspectPayload(payload), "application/vnd.cncf.model.manifest.v1+json"; got != want {
		t.Fatalf("unexpected mediaType %q", got)
	}
	if got, want := inspectModelPackSize(payload), int64(40); got != want {
		t.Fatalf("unexpected size %d", got)
	}
}

func TestValidateModelPackPayloadRejectsBrokenManifest(t *testing.T) {
	t.Parallel()

	err := validateModelPackPayload(map[string]any{
		"digest": "sha256:deadbeef",
		"manifest": map[string]any{
			"schemaVersion": float64(2),
			"artifactType":  "application/vnd.oci.image.manifest.v1+json",
			"config": map[string]any{
				"mediaType": "application/vnd.cncf.model.config.v1+json",
				"digest":    "sha256:config",
				"size":      float64(10),
			},
			"layers": []any{
				map[string]any{
					"mediaType": "application/vnd.cncf.model.weight.v1.tar",
					"digest":    "sha256:layer",
					"size":      float64(30),
					"annotations": map[string]any{
						"org.cncf.model.filepath": "model",
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

func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "ca-*.pem")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return file.Name()
}
