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
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type HTTPProbeResult struct {
	Metadata       HTTPMetadata
	ContentLength  int64
	SupportsRanges bool
}

func ProbeHTTPSource(
	ctx context.Context,
	rawURL string,
	caBundle []byte,
	headers map[string]string,
) (HTTPProbeResult, error) {
	if strings.TrimSpace(rawURL) == "" {
		return HTTPProbeResult{}, fmt.Errorf("HTTP URL must not be empty")
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig(caBundle),
		},
	}

	response, err := doHEAD(ctx, client, rawURL, headers)
	if err == nil && response.StatusCode != http.StatusMethodNotAllowed && response.StatusCode != http.StatusNotImplemented {
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return HTTPProbeResult{}, unexpectedStatusError(response, "HTTP source probe")
		}
		return newHTTPProbeResult(rawURL, response, false), nil
	}
	if response != nil {
		response.Body.Close()
	}

	response, err = doRangeGET(ctx, client, rawURL, headers)
	if err != nil {
		return HTTPProbeResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return HTTPProbeResult{}, unexpectedStatusError(response, "HTTP source probe")
	}
	return newHTTPProbeResult(rawURL, response, true), nil
}

func doHEAD(
	ctx context.Context,
	httpClient *http.Client,
	rawURL string,
	headers map[string]string,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return httpClient.Do(request)
}

func doRangeGET(
	ctx context.Context,
	httpClient *http.Client,
	rawURL string,
	headers map[string]string,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Range", "bytes=0-0")
	return httpClient.Do(request)
}

func newHTTPProbeResult(rawURL string, response *http.Response, usedRangeGET bool) HTTPProbeResult {
	return HTTPProbeResult{
		Metadata: HTTPMetadata{
			URL:          rawURL,
			Filename:     filenameFromHTTPResponse(rawURL, response),
			ETag:         response.Header.Get("ETag"),
			LastModified: response.Header.Get("Last-Modified"),
			ContentType:  response.Header.Get("Content-Type"),
		},
		ContentLength:  responseContentLength(response),
		SupportsRanges: usedRangeGET || strings.EqualFold(strings.TrimSpace(response.Header.Get("Accept-Ranges")), "bytes"),
	}
}

func responseContentLength(response *http.Response) int64 {
	if response == nil {
		return 0
	}
	if value := strings.TrimSpace(response.Header.Get("Content-Range")); value != "" {
		parts := strings.Split(value, "/")
		if len(parts) == 2 {
			size, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if err == nil && size >= 0 {
				return size
			}
		}
	}
	if value := strings.TrimSpace(response.Header.Get("Content-Length")); value != "" {
		size, err := strconv.ParseInt(value, 10, 64)
		if err == nil && size >= 0 {
			return size
		}
	}
	return 0
}
