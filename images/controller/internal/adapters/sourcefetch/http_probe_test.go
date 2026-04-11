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

package sourcefetch

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeHTTPSource(t *testing.T) {
	t.Parallel()

	t.Run("head request succeeds", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodHead {
				t.Fatalf("unexpected method %s", request.Method)
			}
			writer.Header().Set("Content-Length", "128")
			writer.Header().Set("Content-Type", "application/octet-stream")
			writer.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		result, err := ProbeHTTPSource(t.Context(), server.URL+"/model.gguf", nil, nil)
		if err != nil {
			t.Fatalf("ProbeHTTPSource() error = %v", err)
		}
		if got, want := result.Metadata.Filename, "model.gguf"; got != want {
			t.Fatalf("unexpected filename %q", got)
		}
		if got, want := result.ContentLength, int64(128); got != want {
			t.Fatalf("unexpected content length %d", got)
		}
	})

	t.Run("range get fallback works when head is unsupported", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.Method {
			case http.MethodHead:
				writer.WriteHeader(http.StatusMethodNotAllowed)
			case http.MethodGet:
				if got, want := request.Header.Get("Range"), "bytes=0-0"; got != want {
					t.Fatalf("unexpected range header %q", got)
				}
				writer.Header().Set("Content-Range", "bytes 0-0/256")
				writer.Header().Set("Content-Type", "application/octet-stream")
				writer.WriteHeader(http.StatusPartialContent)
				_, _ = writer.Write([]byte("G"))
			default:
				t.Fatalf("unexpected method %s", request.Method)
			}
		}))
		defer server.Close()

		result, err := ProbeHTTPSource(t.Context(), server.URL+"/download.bin", nil, nil)
		if err != nil {
			t.Fatalf("ProbeHTTPSource() error = %v", err)
		}
		if !result.SupportsRanges {
			t.Fatal("expected range support")
		}
		if got, want := result.ContentLength, int64(256); got != want {
			t.Fatalf("unexpected content length %d", got)
		}
	})
}
