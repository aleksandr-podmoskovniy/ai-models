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
	"net/http"
	"net/http/httptest"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestDirectUploadClientRetriesServiceUnavailableAPIResponse(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		if request.URL.Path != "/v2/blob-uploads/complete" {
			http.NotFound(writer, request)
			return
		}
		if user, pass, ok := request.BasicAuth(); !ok || user != "writer" || pass != "secret" {
			t.Fatalf("unexpected direct upload auth %q/%q", user, pass)
		}
		if attempts < 3 {
			http.Error(writer, "temporarily read-only", http.StatusServiceUnavailable)
			return
		}
		writeJSON(writer, completeDirectUploadResponse{
			OK:        true,
			Digest:    "sha256:abc",
			SizeBytes: 1,
		})
	}))
	t.Cleanup(server.Close)

	client := &directUploadClient{
		apiClient: server.Client(),
		endpoint:  server.URL,
		auth: modelpackports.RegistryAuth{
			Username: "writer",
			Password: "secret",
		},
	}

	result, err := client.complete(context.Background(), "session-a", "sha256:abc", 1, nil)
	if err != nil {
		t.Fatalf("complete() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("direct upload API attempts = %d, want 3", attempts)
	}
	if result.Digest != "sha256:abc" || result.SizeBytes != 1 {
		t.Fatalf("unexpected complete result %#v", result)
	}
}
