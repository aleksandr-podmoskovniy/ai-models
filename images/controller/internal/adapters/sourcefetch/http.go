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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type HTTPMetadata struct {
	URL          string
	Filename     string
	ETag         string
	LastModified string
	ContentType  string
}

func (m HTTPMetadata) ResolvedRevision() string {
	switch {
	case strings.TrimSpace(m.ETag) != "":
		return "etag:" + strings.TrimSpace(m.ETag)
	case strings.TrimSpace(m.LastModified) != "":
		return "last-modified:" + strings.TrimSpace(m.LastModified)
	default:
		return ""
	}
}

func DownloadHTTPSource(
	ctx context.Context,
	rawURL string,
	caBundle []byte,
	authDir string,
	destination string,
) (string, HTTPMetadata, error) {
	if strings.TrimSpace(rawURL) == "" {
		return "", HTTPMetadata{}, errors.New("HTTP URL must not be empty")
	}
	if strings.TrimSpace(destination) == "" {
		return "", HTTPMetadata{}, errors.New("HTTP download destination must not be empty")
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", HTTPMetadata{}, err
	}

	headers, err := HTTPAuthHeadersFromDir(authDir)
	if err != nil {
		return "", HTTPMetadata{}, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig(caBundle),
		},
	}
	response, err := doGET(ctx, client, rawURL, headers)
	if err != nil {
		return "", HTTPMetadata{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", HTTPMetadata{}, unexpectedStatusError(response, "HTTP source")
	}

	filename := filenameFromHTTPResponse(rawURL, response)
	sourcePath := filepath.Join(destination, filename)
	if err := writeResponseBody(sourcePath, response.Body); err != nil {
		return "", HTTPMetadata{}, err
	}

	return sourcePath, HTTPMetadata{
		URL:          rawURL,
		Filename:     filename,
		ETag:         response.Header.Get("ETag"),
		LastModified: response.Header.Get("Last-Modified"),
		ContentType:  response.Header.Get("Content-Type"),
	}, nil
}

func fetchHTTPModel(ctx context.Context, options RemoteOptions) (RemoteResult, error) {
	var (
		sourcePath    string
		metadata      HTTPMetadata
		stagedObjects []cleanuphandle.UploadStagingHandle
		err           error
	)
	if rawStageEnabled(options.RawStage) {
		var handle cleanuphandle.UploadStagingHandle
		handle, metadata, err = StageHTTPSource(ctx, options.URL, options.HTTPCABundle, options.HTTPAuthDir, *options.RawStage)
		if err != nil {
			return RemoteResult{}, err
		}
		sourcePath = filepath.Join(options.Workspace, ".raw", handle.FileName)
		if err := downloadStagedObject(ctx, options.RawStage.Client, handle, sourcePath); err != nil {
			return RemoteResult{}, err
		}
		stagedObjects = append(stagedObjects, handle)
	} else {
		sourcePath, metadata, err = DownloadHTTPSource(
			ctx,
			options.URL,
			options.HTTPCABundle,
			options.HTTPAuthDir,
			filepath.Join(options.Workspace, ".download"),
		)
		if err != nil {
			return RemoteResult{}, err
		}
	}

	modelDir, err := PrepareModelInput(sourcePath, filepath.Join(options.Workspace, "checkpoint"))
	if err != nil {
		return RemoteResult{}, err
	}

	inputFormat, err := resolveDirFormat(modelDir, options.RequestedFormat)
	if err != nil {
		return RemoteResult{}, err
	}
	if err := modelformat.ValidateDir(modelDir, inputFormat); err != nil {
		return RemoteResult{}, err
	}

	return RemoteResult{
		SourceType:  modelsv1alpha1.ModelSourceTypeHTTP,
		ModelDir:    modelDir,
		InputFormat: inputFormat,
		Provenance: RemoteProvenance{
			ExternalReference: options.URL,
			ResolvedRevision:  metadata.ResolvedRevision(),
		},
		StagedObjects: stagedObjects,
	}, nil
}

func StageHTTPSource(
	ctx context.Context,
	rawURL string,
	caBundle []byte,
	authDir string,
	rawStage RawStageOptions,
) (cleanuphandle.UploadStagingHandle, HTTPMetadata, error) {
	if !rawStageEnabled(&rawStage) {
		return cleanuphandle.UploadStagingHandle{}, HTTPMetadata{}, errors.New("HTTP raw stage options must not be empty")
	}

	headers, err := HTTPAuthHeadersFromDir(authDir)
	if err != nil {
		return cleanuphandle.UploadStagingHandle{}, HTTPMetadata{}, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig(caBundle),
		},
	}
	response, err := doGET(ctx, client, rawURL, headers)
	if err != nil {
		return cleanuphandle.UploadStagingHandle{}, HTTPMetadata{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return cleanuphandle.UploadStagingHandle{}, HTTPMetadata{}, unexpectedStatusError(response, "HTTP source")
	}

	filename := filenameFromHTTPResponse(rawURL, response)
	handle, err := stageRawObject(
		ctx,
		rawStage,
		filename,
		filename,
		response.ContentLength,
		response.Header.Get("Content-Type"),
		response.Body,
	)
	if err != nil {
		return cleanuphandle.UploadStagingHandle{}, HTTPMetadata{}, err
	}

	return handle, HTTPMetadata{
		URL:          rawURL,
		Filename:     filename,
		ETag:         response.Header.Get("ETag"),
		LastModified: response.Header.Get("Last-Modified"),
		ContentType:  response.Header.Get("Content-Type"),
	}, nil
}

func DecodeInlineCABundle(raw string) ([]byte, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode HTTP CA bundle: %w", err)
	}
	return decoded, nil
}

func HTTPAuthHeadersFromDir(authDir string) (map[string]string, error) {
	if strings.TrimSpace(authDir) == "" {
		return map[string]string{}, nil
	}

	root := filepath.Clean(authDir)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("HTTP auth directory does not exist: %s", authDir)
	}

	if rawAuthorization, err := os.ReadFile(filepath.Join(root, "authorization")); err == nil {
		value := strings.TrimSpace(string(rawAuthorization))
		if value != "" {
			return map[string]string{"Authorization": value}, nil
		}
	}

	username, userErr := os.ReadFile(filepath.Join(root, "username"))
	password, passErr := os.ReadFile(filepath.Join(root, "password"))
	if userErr == nil && passErr == nil {
		token := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(string(username)) + ":" + strings.TrimSpace(string(password))))
		return map[string]string{"Authorization": "Basic " + token}, nil
	}

	return map[string]string{}, nil
}

func filenameFromHTTPResponse(rawURL string, response *http.Response) string {
	if contentDisposition := strings.TrimSpace(response.Header.Get("Content-Disposition")); contentDisposition != "" {
		if _, params, err := mime.ParseMediaType(contentDisposition); err == nil {
			if filename := strings.TrimSpace(params["filename"]); filename != "" {
				return filepath.Base(filename)
			}
			if filename := strings.TrimSpace(params["filename*"]); filename != "" {
				return filepath.Base(filename)
			}
		}
	}

	parsed, err := url.Parse(rawURL)
	if err == nil {
		if base := filepath.Base(parsed.Path); base != "" && base != "." && base != "/" {
			return base
		}
	}
	return "model-download"
}

func tlsConfig(caBundle []byte) *tls.Config {
	if len(caBundle) == 0 {
		return &tls.Config{}
	}

	pool, err := x509.SystemCertPool()
	if pool == nil || err != nil {
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM(caBundle)
	return &tls.Config{RootCAs: pool}
}
