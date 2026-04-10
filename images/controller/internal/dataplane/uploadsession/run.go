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
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

const uploadFilenameHeader = "X-AI-MODELS-FILENAME"

type Options struct {
	ListenPort        int
	UploadToken       string
	ExpectedSizeBytes int64
	StagingBucket     string
	StagingKeyPrefix  string
	StagingUploader   uploadstagingports.Uploader
}

func Run(ctx context.Context, options Options) (cleanuphandle.Handle, error) {
	if strings.TrimSpace(options.UploadToken) == "" {
		return cleanuphandle.Handle{}, errors.New("upload token must not be empty")
	}
	if strings.TrimSpace(options.StagingBucket) == "" {
		return cleanuphandle.Handle{}, errors.New("staging bucket must not be empty")
	}
	if strings.TrimSpace(options.StagingKeyPrefix) == "" {
		return cleanuphandle.Handle{}, errors.New("staging key prefix must not be empty")
	}
	if options.StagingUploader == nil {
		return cleanuphandle.Handle{}, errors.New("staging uploader must not be nil")
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", normalizePort(options.ListenPort)),
	}
	resultCh := make(chan runResult, 1)
	server.Handler = newHandler(options, resultCh)

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
		return cleanuphandle.Handle{}, ctx.Err()
	case result := <-resultCh:
		_ = server.Shutdown(context.Background())
		if result.err != nil {
			return cleanuphandle.Handle{}, result.err
		}
		if err := <-serverErrCh; err != nil {
			return cleanuphandle.Handle{}, err
		}
		return result.value, nil
	case err := <-serverErrCh:
		return cleanuphandle.Handle{}, err
	}
}

type runResult struct {
	value cleanuphandle.Handle
	err   error
}

func newHandler(options Options, resultCh chan<- runResult) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/upload", func(writer http.ResponseWriter, request *http.Request) {
		handleUpload(writer, request, options, resultCh)
	})
	mux.HandleFunc("/upload/", func(writer http.ResponseWriter, request *http.Request) {
		handleUpload(writer, request, options, resultCh)
	})
	return mux
}

func handleUpload(
	writer http.ResponseWriter,
	request *http.Request,
	options Options,
	resultCh chan<- runResult,
) {
	if request.Method != http.MethodPut {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !matchesUploadToken(request, options.UploadToken) {
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

	fileName := sanitizedUploadFileName(request.Header.Get(uploadFilenameHeader))
	key, err := uploadKey(options.StagingKeyPrefix, fileName)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		resultCh <- runResult{err: err}
		return
	}

	if err := options.StagingUploader.Upload(request.Context(), uploadstagingports.UploadInput{
		Bucket:        options.StagingBucket,
		Key:           key,
		ContentLength: length,
		Body:          request.Body,
	}); err != nil {
		http.Error(writer, "upload staging failed", http.StatusInternalServerError)
		resultCh <- runResult{err: err}
		return
	}

	result := cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    options.StagingBucket,
			Key:       key,
			FileName:  fileName,
			SizeBytes: length,
		},
	}
	writer.WriteHeader(http.StatusCreated)
	_, _ = writer.Write([]byte("upload accepted\n"))
	resultCh <- runResult{value: result}
}

func matchesUploadToken(request *http.Request, expectedToken string) bool {
	expectedToken = strings.TrimSpace(expectedToken)
	if expectedToken == "" {
		return false
	}
	if token, ok := uploadTokenFromPath(request.URL.Path); ok {
		return subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1
	}
	auth := strings.TrimSpace(request.Header.Get("Authorization"))
	if auth == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(auth), []byte("Bearer "+expectedToken)) == 1
}

func uploadTokenFromPath(path string) (string, bool) {
	if !strings.HasPrefix(path, "/upload/") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(path, "/upload/"))
	if token == "" || strings.Contains(token, "/") {
		return "", false
	}
	return token, true
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

func uploadKey(prefix string, fileName string) (string, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return "", errors.New("upload staging key prefix must not be empty")
	}
	return prefix + "/" + fileName, nil
}

func normalizePort(port int) int {
	if port > 0 {
		return port
	}
	return 8444
}
