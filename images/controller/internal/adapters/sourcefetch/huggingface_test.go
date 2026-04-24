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

func TestHuggingFaceInfoURL(t *testing.T) {
	endpoint, err := huggingFaceInfoURL("deepseek-ai/DeepSeek-R1", "main")
	if err != nil {
		t.Fatalf("huggingFaceInfoURL() error = %v", err)
	}
	if got, want := endpoint, "https://huggingface.co/api/models/deepseek-ai/DeepSeek-R1?revision=main"; got != want {
		t.Fatalf("unexpected endpoint %q", got)
	}
}

func TestResolvedHuggingFaceRevision(t *testing.T) {
	t.Parallel()

	if got, want := ResolveHuggingFaceRevision(HuggingFaceInfo{SHA: "deadbeef"}, "main"), "deadbeef"; got != want {
		t.Fatalf("unexpected revision %q", got)
	}
}

func TestFetchHuggingFaceInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got, want := request.URL.Path, "/api/models/deepseek-ai/DeepSeek-R1"; got != want {
			t.Fatalf("unexpected path %q", got)
		}
		if got, want := request.URL.Query().Get("revision"), "main"; got != want {
			t.Fatalf("unexpected revision query %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
		  "id": "deepseek-ai/DeepSeek-R1",
		  "sha": "deadbeef",
		  "pipeline_tag": "text-generation",
		  "cardData": {"license": "mit", "base_model": "ignored"},
		  "downloads": 42,
		  "likes": 7,
		  "tags": ["llm", "ignored"],
		  "siblings": [
		    {"rfilename": "config.json"},
		    {"rfilename": "model.safetensors"}
		  ]
		}`))
	}))
	defer server.Close()

	withHuggingFaceBaseURL(t, server.URL)
	info, err := FetchHuggingFaceInfo(t.Context(), "deepseek-ai/DeepSeek-R1", "main", "hf-token")
	if err != nil {
		t.Fatalf("FetchHuggingFaceInfo() error = %v", err)
	}

	if got, want := info.ID, "deepseek-ai/DeepSeek-R1"; got != want {
		t.Fatalf("unexpected ID %q", got)
	}
	if got, want := info.SHA, "deadbeef"; got != want {
		t.Fatalf("unexpected SHA %q", got)
	}
	if got, want := info.PipelineTag, "text-generation"; got != want {
		t.Fatalf("unexpected pipeline tag %q", got)
	}
	if got, want := info.License, "mit"; got != want {
		t.Fatalf("unexpected license %q", got)
	}
	if got, want := len(info.Files), 2; got != want {
		t.Fatalf("unexpected file count %d", got)
	}
}
