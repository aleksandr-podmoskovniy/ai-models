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
	"net/http"
	"strings"
)

type ollamaObjectReader struct {
	httpClient *http.Client
}

func (r ollamaObjectReader) OpenRead(ctx context.Context, sourcePath string) (RemoteOpenReadResult, error) {
	return r.openRead(ctx, sourcePath, 0, -1)
}

func (r ollamaObjectReader) OpenReadRange(ctx context.Context, sourcePath string, offset, length int64) (RemoteOpenReadResult, error) {
	return r.openRead(ctx, sourcePath, offset, length)
}

func (r ollamaObjectReader) openRead(ctx context.Context, sourcePath string, offset, length int64) (RemoteOpenReadResult, error) {
	headers := map[string]string{}
	if rangeHeader, ok := httpByteRangeHeader(offset, length); ok {
		headers["Range"] = rangeHeader
	}
	response, err := doGET(ctx, r.httpClient, strings.TrimSpace(sourcePath), headers)
	if err != nil {
		return RemoteOpenReadResult{}, err
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		defer response.Body.Close()
		return RemoteOpenReadResult{}, unexpectedStatusError(response, "ollama object-source GET request")
	}
	sizeBytes, err := responseBodyLength(response)
	if err != nil {
		_ = response.Body.Close()
		return RemoteOpenReadResult{}, err
	}
	return RemoteOpenReadResult{
		Body:      response.Body,
		SizeBytes: sizeBytes,
		ETag:      strings.TrimSpace(response.Header.Get("ETag")),
	}, nil
}
