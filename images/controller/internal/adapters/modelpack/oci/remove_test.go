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
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestAdapterRemoveTrustsDeleteAcknowledgementWithoutVisibilityProbe(t *testing.T) {
	t.Parallel()

	const digest = "sha256:deadbeef"
	getCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user, pass, ok := request.BasicAuth()
		if !ok || user != "writer" || pass != "secret" {
			t.Fatalf("unexpected auth %q/%q", user, pass)
		}
		if got, want := request.Header.Get("Accept"), ManifestAcceptHeader; got != want {
			t.Fatalf("unexpected Accept header %q", got)
		}
		switch {
		case request.Method == http.MethodDelete && request.URL.Path == "/v2/ai-models/catalog/model/manifests/"+digest:
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodGet && request.URL.Path == "/v2/ai-models/catalog/model/manifests/"+digest:
			getCalls++
			t.Fatalf("Remove() must not verify successful delete with expected 404-producing GET")
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	auth := modelpackports.RegistryAuth{
		Username: "writer",
		Password: "secret",
		CAFile: writeTempFile(t, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: server.Certificate().Raw,
		})),
	}
	adapter := New()

	if err := adapter.Remove(context.Background(), immutableOCIReference(serverReference(server, "published"), digest), auth); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if getCalls != 0 {
		t.Fatalf("manifest verification GET calls = %d, want 0", getCalls)
	}
}

func TestAdapterRemoveTreatsMissingManifestAsDeleted(t *testing.T) {
	t.Parallel()

	const digest = "sha256:deadbeef"
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user, pass, ok := request.BasicAuth()
		if !ok || user != "writer" || pass != "secret" {
			t.Fatalf("unexpected auth %q/%q", user, pass)
		}
		if got, want := request.Header.Get("Accept"), ManifestAcceptHeader; got != want {
			t.Fatalf("unexpected Accept header %q", got)
		}
		switch {
		case request.Method == http.MethodDelete && request.URL.Path == "/v2/ai-models/catalog/model/manifests/"+digest:
			writer.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	auth := modelpackports.RegistryAuth{
		Username: "writer",
		Password: "secret",
		CAFile: writeTempFile(t, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: server.Certificate().Raw,
		})),
	}
	adapter := New()

	if err := adapter.Remove(context.Background(), immutableOCIReference(serverReference(server, "published"), digest), auth); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
}
