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
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"syscall"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestDirectUploadClientRetriesServiceUnavailableAPIResponse(t *testing.T) {
	t.Parallel()

	client, attempts := newCompleteDirectUploadTestClient(t, func(writer http.ResponseWriter, _ *http.Request, attempt int) {
		if attempt < 3 {
			http.Error(writer, "temporarily read-only", http.StatusServiceUnavailable)
			return
		}
		writeJSON(writer, validCompleteDirectUploadResponse())
	})

	result, err := client.complete(context.Background(), "session-a", "sha256:abc", 1, nil)
	if err != nil {
		t.Fatalf("complete() error = %v", err)
	}
	if *attempts != 3 {
		t.Fatalf("direct upload API attempts = %d, want 3", *attempts)
	}
	if result.Digest != "sha256:abc" || result.SizeBytes != 1 {
		t.Fatalf("unexpected complete result %#v", result)
	}
}

func TestDirectUploadClientRetriesTransientAPIResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "internal server error", statusCode: http.StatusInternalServerError},
		{name: "bad gateway", statusCode: http.StatusBadGateway},
		{name: "service unavailable", statusCode: http.StatusServiceUnavailable},
		{name: "gateway timeout", statusCode: http.StatusGatewayTimeout},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client, attempts := newCompleteDirectUploadTestClient(t, func(writer http.ResponseWriter, _ *http.Request, attempt int) {
				if attempt == 1 {
					http.Error(writer, "temporary backend failure", test.statusCode)
					return
				}
				writeJSON(writer, validCompleteDirectUploadResponse())
			})

			result, err := client.complete(context.Background(), "session-a", "sha256:abc", 1, nil)
			if err != nil {
				t.Fatalf("complete() error = %v", err)
			}
			if *attempts != 2 {
				t.Fatalf("direct upload API attempts = %d, want 2", *attempts)
			}
			if result.Digest != "sha256:abc" || result.SizeBytes != 1 {
				t.Fatalf("unexpected complete result %#v", result)
			}
		})
	}
}

func TestDirectUploadClientRetriesTransportError(t *testing.T) {
	t.Parallel()

	attempts := 0
	client := newDirectUploadClientForTest(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		if request.URL.Path != "/v2/blob-uploads/complete" {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     make(http.Header),
				Request:    request,
			}, nil
		}
		if attempts == 1 {
			return nil, &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"ok": true,
				"digest": "sha256:abc",
				"sizeBytes": 1
			}`)),
			Header:  make(http.Header),
			Request: request,
		}, nil
	})}, "https://direct-upload.example.test")

	result, err := client.complete(context.Background(), "session-a", "sha256:abc", 1, nil)
	if err != nil {
		t.Fatalf("complete() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("direct upload API attempts = %d, want 2", attempts)
	}
	if result.Digest != "sha256:abc" || result.SizeBytes != 1 {
		t.Fatalf("unexpected complete result %#v", result)
	}
}

func TestDirectUploadClientRejectsMalformedCompleteSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response completeDirectUploadResponse
	}{
		{
			name: "not ok",
			response: completeDirectUploadResponse{
				OK:        false,
				Digest:    "sha256:abc",
				SizeBytes: 1,
			},
		},
		{
			name: "missing digest",
			response: completeDirectUploadResponse{
				OK:        true,
				SizeBytes: 1,
			},
		},
		{
			name: "non-positive size",
			response: completeDirectUploadResponse{
				OK:        true,
				Digest:    "sha256:abc",
				SizeBytes: 0,
			},
		},
		{
			name: "wrong digest",
			response: completeDirectUploadResponse{
				OK:        true,
				Digest:    "sha256:def",
				SizeBytes: 1,
			},
		},
		{
			name: "wrong size",
			response: completeDirectUploadResponse{
				OK:        true,
				Digest:    "sha256:abc",
				SizeBytes: 2,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newCompleteDirectUploadTestClient(t, func(writer http.ResponseWriter, _ *http.Request, _ int) {
				writeJSON(writer, test.response)
			})

			_, err := client.complete(context.Background(), "session-a", "sha256:abc", 1, nil)
			if err == nil {
				t.Fatal("complete() error = nil, want malformed success rejection")
			}
		})
	}
}

func TestDirectUploadClientDoesNotRetryPermanentTransportError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "certificate authority",
			err: &url.Error{
				Op:  "Get",
				URL: "https://direct-upload.example.test",
				Err: x509.UnknownAuthorityError{},
			},
		},
		{
			name: "dns not found",
			err: &url.Error{
				Op:  "Get",
				URL: "https://direct-upload.example.test",
				Err: &net.DNSError{Err: "no such host", Name: "direct-upload.example.test", IsNotFound: true},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			attempts := 0
			client := newDirectUploadClientForTest(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				attempts++
				return nil, test.err
			})}, "https://direct-upload.example.test")

			_, err := client.complete(context.Background(), "session-a", "sha256:abc", 1, nil)
			if err == nil {
				t.Fatal("complete() error = nil, want permanent transport error")
			}
			if attempts != 1 {
				t.Fatalf("direct upload API attempts = %d, want 1", attempts)
			}
		})
	}
}

func TestDirectUploadClientDoesNotRetryTerminalAPIResponse(t *testing.T) {
	t.Parallel()

	client, attempts := newCompleteDirectUploadTestClient(t, func(writer http.ResponseWriter, _ *http.Request, _ int) {
		http.Error(writer, "bad session", http.StatusBadRequest)
	})

	_, err := client.complete(context.Background(), "session-a", "sha256:abc", 1, nil)
	if err == nil {
		t.Fatal("complete() error = nil, want terminal bad request")
	}
	if *attempts != 1 {
		t.Fatalf("direct upload API attempts = %d, want 1", *attempts)
	}
}

func newCompleteDirectUploadTestClient(
	t *testing.T,
	handle func(http.ResponseWriter, *http.Request, int),
) (*directUploadClient, *int) {
	t.Helper()

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
		handle(writer, request, attempts)
	}))
	t.Cleanup(server.Close)

	return newDirectUploadClientForTest(server.Client(), server.URL), &attempts
}

func validCompleteDirectUploadResponse() completeDirectUploadResponse {
	return completeDirectUploadResponse{
		OK:        true,
		Digest:    "sha256:abc",
		SizeBytes: 1,
	}
}

func newDirectUploadClientForTest(client *http.Client, endpoint string) *directUploadClient {
	return &directUploadClient{
		apiClient: client,
		endpoint:  endpoint,
		auth: modelpackports.RegistryAuth{
			Username: "writer",
			Password: "secret",
		},
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
