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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func doGET(
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
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return httpClient.Do(request)
}

func decodeJSONResponse(response *http.Response, into any) error {
	return json.NewDecoder(response.Body).Decode(into)
}

func unexpectedStatusError(response *http.Response, subject string) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s returned status %d", subject, response.StatusCode)
	}
	return fmt.Errorf("%s returned status %d: %s", subject, response.StatusCode, message)
}

func writeResponseBody(target string, body io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	stream, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(stream, body); err != nil {
		stream.Close()
		return err
	}
	return stream.Close()
}

func bearerAuthHeaders(token string) map[string]string {
	token = strings.TrimSpace(token)
	if token == "" {
		return map[string]string{}
	}
	return map[string]string{
		"Authorization": "Bearer " + token,
	}
}
