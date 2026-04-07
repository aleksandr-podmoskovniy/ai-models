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
	"os"
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestFetchRemoteModelHTTPGGUF(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/model.gguf" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte("GGUFpayload"))
	}))
	defer server.Close()

	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:       server.URL + "/model.gguf",
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}
	if got, want := result.SourceType, modelsv1alpha1.ModelSourceTypeHTTP; got != want {
		t.Fatalf("unexpected source type %q", got)
	}
	if got, want := result.InputFormat, modelsv1alpha1.ModelInputFormatGGUF; got != want {
		t.Fatalf("unexpected input format %q", got)
	}
	if _, err := os.Stat(filepath.Join(result.ModelDir, "model.gguf")); err != nil {
		t.Fatalf("Stat(model.gguf) error = %v", err)
	}
}
