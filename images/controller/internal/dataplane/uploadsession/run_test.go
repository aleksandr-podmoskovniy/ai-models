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

package uploadsession

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func TestRunRejectsMissingUploadToken(t *testing.T) {
	t.Parallel()

	_, err := Run(t.Context(), Options{
		StagingBucket:    "ai-models",
		StagingKeyPrefix: "uploaded-model-staging/1111-2222",
		StagingUploader:  fakeUploader{},
	})
	if err == nil || !strings.Contains(err.Error(), "upload token") {
		t.Fatalf("expected upload token validation error, got %v", err)
	}
}

func TestRunRejectsMissingStagingBucket(t *testing.T) {
	t.Parallel()

	_, err := Run(t.Context(), Options{
		UploadToken:      "token",
		StagingKeyPrefix: "uploaded-model-staging/1111-2222",
		StagingUploader:  fakeUploader{},
	})
	if err == nil || !strings.Contains(err.Error(), "staging bucket") {
		t.Fatalf("expected staging bucket validation error, got %v", err)
	}
}

func TestHandlerExposesHealthz(t *testing.T) {
	t.Parallel()

	handler := newHandler(Options{
		UploadToken:      "token",
		StagingBucket:    "ai-models",
		StagingKeyPrefix: "uploaded-model-staging/1111-2222",
		StagingUploader:  fakeUploader{},
	}, make(chan runResult, 1))

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("unexpected status %d", got)
	}
}

func TestHandlerRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	handler := newHandler(Options{
		UploadToken:      "token",
		StagingBucket:    "ai-models",
		StagingKeyPrefix: "uploaded-model-staging/1111-2222",
		StagingUploader:  fakeUploader{},
	}, make(chan runResult, 1))

	request := httptest.NewRequest(http.MethodPut, "/upload", strings.NewReader("payload"))
	request.Header.Set("Authorization", "Bearer wrong")
	request.Header.Set("Content-Length", "7")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("unexpected status %d", got)
	}
}

func TestHandlerStagesUploadedPayload(t *testing.T) {
	t.Parallel()

	uploader := &recordingUploader{}
	handler := newHandler(Options{
		UploadToken:      "token",
		StagingBucket:    "ai-models",
		StagingKeyPrefix: "uploaded-model-staging/1111-2222",
		StagingUploader:  uploader,
	}, make(chan runResult, 1))

	body := []byte("GGUFpayload")
	request := httptest.NewRequest(http.MethodPut, "/upload", bytes.NewReader(body))
	request = request.WithContext(context.Background())
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Length", strconv.Itoa(len(body)))
	request.Header.Set(uploadFilenameHeader, "model.gguf")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if uploader.bucket != "ai-models" {
		t.Fatalf("unexpected bucket %q", uploader.bucket)
	}
	if uploader.key != "uploaded-model-staging/1111-2222/model.gguf" {
		t.Fatalf("unexpected key %q", uploader.key)
	}
	if !bytes.Equal(uploader.payload, body) {
		t.Fatalf("unexpected payload %q", string(uploader.payload))
	}
}

func TestHandlerPreservesUploadedArchiveFileName(t *testing.T) {
	t.Parallel()

	uploader := &recordingUploader{}
	handler := newHandler(Options{
		UploadToken:      "token",
		StagingBucket:    "ai-models",
		StagingKeyPrefix: "uploaded-model-staging/1111-2222",
		StagingUploader:  uploader,
	}, make(chan runResult, 1))

	body := []byte("archive")
	request := httptest.NewRequest(http.MethodPut, "/upload", bytes.NewReader(body))
	request = request.WithContext(context.Background())
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Length", strconv.Itoa(len(body)))
	request.Header.Set(uploadFilenameHeader, "model.tar")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusCreated; got != want {
		t.Fatalf("unexpected status %d: %s", got, response.Body.String())
	}
	if uploader.key != "uploaded-model-staging/1111-2222/model.tar" {
		t.Fatalf("unexpected key %q", uploader.key)
	}
}

func TestSanitizedUploadFileName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "upload.bin"},
		{name: "basename", input: "model.tar.gz", want: "model.tar.gz"},
		{name: "path", input: "/tmp/model.gguf", want: "model.gguf"},
		{name: "windows path", input: `C:\tmp\model.gguf`, want: "model.gguf"},
		{name: "hidden", input: ".env", want: "upload.bin"},
		{name: "parent", input: "../evil.tar", want: "evil.tar"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizedUploadFileName(tc.input); got != tc.want {
				t.Fatalf("sanitizedUploadFileName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizePortDefaults(t *testing.T) {
	t.Parallel()

	if got, want := normalizePort(0), 8444; got != want {
		t.Fatalf("normalizePort(0) = %d, want %d", got, want)
	}
	if got, want := normalizePort(18080), 18080; got != want {
		t.Fatalf("normalizePort(18080) = %d, want %d", got, want)
	}
}

type fakeUploader struct{}

func (fakeUploader) Upload(context.Context, uploadstagingports.UploadInput) error {
	return nil
}

type recordingUploader struct {
	bucket  string
	key     string
	payload []byte
}

func (r *recordingUploader) Upload(_ context.Context, input uploadstagingports.UploadInput) error {
	payload, err := io.ReadAll(input.Body)
	if err != nil {
		return err
	}
	r.bucket = input.Bucket
	r.key = input.Key
	r.payload = payload
	return nil
}
