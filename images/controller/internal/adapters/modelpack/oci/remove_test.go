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
	"strings"
	"testing"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestAdapterRemoveFailsWhenRegistryKeepsManifestVisibleAfterDelete(t *testing.T) {
	t.Parallel()

	const digest = "sha256:deadbeef"
	manifestPayload := []byte(`{"schemaVersion":2}`)
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
			writer.Header().Set("Content-Type", ManifestMediaType)
			writer.Header().Set("Docker-Content-Digest", digest)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(manifestPayload)
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
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	err := adapter.Remove(ctx, immutableOCIReference(serverReference(server, "published"), digest), auth)
	if err == nil || !strings.Contains(err.Error(), "still exists after delete acknowledgement") {
		t.Fatalf("Remove() error = %v, want post-delete visibility failure", err)
	}
}
