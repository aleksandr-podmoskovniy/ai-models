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
	"archive/tar"
	"bytes"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type modelPackServerOptions struct {
	layerTar     []byte
	layerHook    func()
	manifestBody []byte
	configBody   []byte
	blobBodies   map[string][]byte
	blobHooks    map[string]func()
	blobHandlers map[string]func(http.ResponseWriter, *http.Request, []byte) bool
}

func newModelPackTestServer(t *testing.T, options modelPackServerOptions) (*httptest.Server, modelpackports.RegistryAuth, string) {
	t.Helper()

	manifestBody, configBody, blobBodies := defaultModelPackServerPayload(t, options)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/ai-models/catalog/model/manifests/published", r.URL.Path == "/v2/ai-models/catalog/model/manifests/sha256:deadbeef":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "writer" || pass != "secret" {
				t.Fatalf("unexpected auth %q/%q", user, pass)
			}
			w.Header().Set("Docker-Content-Digest", "sha256:deadbeef")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(manifestBody)
		case r.URL.Path == "/v2/ai-models/catalog/model/blobs/sha256:config":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "writer" || pass != "secret" {
				t.Fatalf("unexpected auth %q/%q", user, pass)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(configBody)
		case strings.HasPrefix(r.URL.Path, "/v2/ai-models/catalog/model/blobs/"):
			digest := strings.TrimPrefix(r.URL.Path, "/v2/ai-models/catalog/model/blobs/")
			if hook := options.blobHooks[digest]; hook != nil {
				hook()
			}
			if digest == "sha256:layer" && options.layerHook != nil {
				options.layerHook()
			}
			payload, ok := blobBodies[digest]
			if !ok {
				t.Fatalf("unexpected blob digest %q", digest)
			}
			user, pass, ok := r.BasicAuth()
			if !ok || user != "writer" || pass != "secret" {
				t.Fatalf("unexpected auth %q/%q", user, pass)
			}
			if handler := options.blobHandlers[digest]; handler != nil && handler(w, r, payload) {
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	caFile := writeTempFile(t, certPEM)
	return server, modelpackports.RegistryAuth{
		Username: "writer",
		Password: "secret",
		CAFile:   caFile,
	}, caFile
}

func defaultModelPackServerPayload(
	t *testing.T,
	options modelPackServerOptions,
) ([]byte, []byte, map[string][]byte) {
	t.Helper()

	if len(options.manifestBody) > 0 && len(options.configBody) > 0 && len(options.blobBodies) > 0 {
		return options.manifestBody, options.configBody, options.blobBodies
	}

	manifestBody, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"artifactType":  ModelPackArtifactType,
		"config": map[string]any{
			"mediaType": ModelPackConfigMediaType,
			"digest":    "sha256:config",
			"size":      10,
		},
		"layers": []map[string]any{
			{
				"mediaType": ModelPackWeightLayerType,
				"digest":    "sha256:layer",
				"size":      103,
				"annotations": map[string]string{
					ModelPackFilepathAnnotation: "model",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal(manifest) error = %v", err)
	}

	configBody, err := json.Marshal(map[string]any{
		"descriptor": map[string]any{"name": "model"},
		"modelfs": map[string]any{
			"type":    "layers",
			"diffIds": []string{"sha256:layer-diff"},
		},
		"config": map[string]any{},
	})
	if err != nil {
		t.Fatalf("json.Marshal(config) error = %v", err)
	}

	return manifestBody, configBody, map[string][]byte{
		"sha256:layer": options.layerTar,
	}
}

func serverReference(server *httptest.Server, tag string) string {
	base := strings.TrimPrefix(server.URL, "https://") + "/ai-models/catalog/model"
	if strings.HasPrefix(tag, "@") {
		return base + tag
	}
	return base + ":" + tag
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

func tarBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader() error = %v", err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buffer.Bytes()
}
