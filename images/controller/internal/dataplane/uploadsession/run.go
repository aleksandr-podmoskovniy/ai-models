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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/artifactbackend"
	"github.com/deckhouse/ai-models/controller/internal/dataplane/publishworker"
)

const uploadFilenameHeader = "X-AI-MODELS-FILENAME"

type Options struct {
	ListenPort        int
	UploadToken       string
	ExpectedSizeBytes int64
	InputFormat       modelsv1alpha1.ModelInputFormat
	Publish           publishworker.Options
}

func Run(ctx context.Context, options Options) (artifactbackend.Result, error) {
	if strings.TrimSpace(options.UploadToken) == "" {
		return artifactbackend.Result{}, errors.New("upload token must not be empty")
	}
	if strings.TrimSpace(options.Publish.Task) == "" {
		return artifactbackend.Result{}, errors.New("task is required for upload session")
	}

	uploadDir, err := os.MkdirTemp("", "ai-model-upload-session-")
	if err != nil {
		return artifactbackend.Result{}, err
	}
	defer os.RemoveAll(uploadDir)

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", normalizePort(options.ListenPort)),
	}
	resultCh := make(chan runResult, 1)
	server.Handler = newHandler(uploadDir, options, resultCh)

	serverErrCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
			return
		}
		serverErrCh <- nil
	}()

	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return artifactbackend.Result{}, ctx.Err()
	case result := <-resultCh:
		_ = server.Shutdown(context.Background())
		if result.err != nil {
			return artifactbackend.Result{}, result.err
		}
		if err := <-serverErrCh; err != nil {
			return artifactbackend.Result{}, err
		}
		return result.value, nil
	case err := <-serverErrCh:
		return artifactbackend.Result{}, err
	}
}

type runResult struct {
	value artifactbackend.Result
	err   error
}

func newHandler(uploadDir string, options Options, resultCh chan<- runResult) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/upload", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if auth := strings.TrimSpace(request.Header.Get("Authorization")); auth != "Bearer "+strings.TrimSpace(options.UploadToken) {
			http.Error(writer, "invalid upload token", http.StatusUnauthorized)
			return
		}
		contentLength := strings.TrimSpace(request.Header.Get("Content-Length"))
		if contentLength == "" {
			http.Error(writer, "Content-Length header is required", http.StatusLengthRequired)
			return
		}
		length, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil || length <= 0 {
			http.Error(writer, "invalid Content-Length header", http.StatusBadRequest)
			return
		}
		if options.ExpectedSizeBytes > 0 && length != options.ExpectedSizeBytes {
			http.Error(writer, "uploaded payload size does not match expected-size-bytes", http.StatusBadRequest)
			return
		}

		uploadPath := filepath.Join(uploadDir, sanitizedUploadFileName(request.Header.Get(uploadFilenameHeader)))
		stream, err := os.OpenFile(uploadPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			resultCh <- runResult{err: err}
			return
		}
		written, err := io.Copy(stream, io.LimitReader(request.Body, length))
		closeErr := stream.Close()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			resultCh <- runResult{err: err}
			return
		}
		if closeErr != nil {
			http.Error(writer, closeErr.Error(), http.StatusInternalServerError)
			resultCh <- runResult{err: closeErr}
			return
		}
		if written != length {
			err := errors.New("unexpected end of upload stream")
			http.Error(writer, err.Error(), http.StatusBadRequest)
			resultCh <- runResult{err: err}
			return
		}

		publishOptions := options.Publish
		publishOptions.SourceType = modelsv1alpha1.ModelSourceTypeUpload
		publishOptions.UploadPath = uploadPath
		publishOptions.InputFormat = options.InputFormat

		result, err := publishworker.Run(request.Context(), publishOptions)
		if err != nil {
			http.Error(writer, "upload processing failed", http.StatusInternalServerError)
			resultCh <- runResult{err: err}
			return
		}

		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte("upload accepted\n"))
		resultCh <- runResult{value: result}
	})
	return mux
}

func sanitizedUploadFileName(raw string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if trimmed == "" {
		return "upload.bin"
	}

	base := strings.TrimSpace(filepath.Base(trimmed))
	switch base {
	case "", ".", "..", string(filepath.Separator):
		return "upload.bin"
	}
	if strings.HasPrefix(base, ".") {
		return "upload.bin"
	}
	return base
}

func normalizePort(port int) int {
	if port > 0 {
		return port
	}
	return 8444
}
