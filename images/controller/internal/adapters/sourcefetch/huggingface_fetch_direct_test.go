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
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestFetchRemoteModelHuggingFacePlansDirectSafetensorsObjectSourceWithoutMirror(t *testing.T) {
	previousInfoFetcher := fetchHuggingFaceInfoFunc
	previousBaseURL := huggingFaceBaseURL
	t.Cleanup(func() {
		fetchHuggingFaceInfoFunc = previousInfoFetcher
		huggingFaceBaseURL = previousBaseURL
	})

	configPayload := []byte(`{"architectures":["LlamaForCausalLM"]}`)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got, want := request.Header.Get("Authorization"), "Bearer hf-token"; got != want {
			t.Fatalf("unexpected authorization header %q", got)
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/owner/model/resolve/deadbeef/config.json":
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("Content-Length", strconv.Itoa(len(configPayload)))
			writer.Header().Set("ETag", `"etag-config"`)
			_, _ = writer.Write(configPayload)
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/config.json":
			writer.Header().Set("Content-Length", strconv.Itoa(len(configPayload)))
			writer.Header().Set("ETag", `"etag-config"`)
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/model.safetensors":
			writer.Header().Set("Content-Length", "14")
			writer.Header().Set("ETag", `"etag-model"`)
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	fetchHuggingFaceInfoFunc = func(context.Context, string, string, string) (HuggingFaceInfo, error) {
		return HuggingFaceInfo{
			ID:          "owner/model",
			SHA:         "deadbeef",
			PipelineTag: "text-generation",
			License:     "apache-2.0",
			Files:       []string{"config.json", "model.safetensors"},
		}, nil
	}
	huggingFaceBaseURL = server.URL

	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:                      "https://huggingface.co/owner/model?revision=main",
		HFToken:                  "hf-token",
		SkipLocalMaterialization: true,
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}

	if got := result.ModelDir; got != "" {
		t.Fatalf("expected no local model dir, got %q", got)
	}
	if result.ObjectSource == nil {
		t.Fatal("expected direct object source")
	}
	if result.ProfileSummary == nil {
		t.Fatal("expected remote profile summary")
	}
	if got, want := result.InputFormat, modelsv1alpha1.ModelInputFormatSafetensors; got != want {
		t.Fatalf("unexpected input format %q", got)
	}
	if got, want := result.Provenance.ResolvedRevision, "deadbeef"; got != want {
		t.Fatalf("unexpected resolved revision %q", got)
	}
	if got, want := len(result.ObjectSource.Files), 2; got != want {
		t.Fatalf("unexpected direct object source file count %d", got)
	}
	if got, want := result.ObjectSource.Files[0].TargetPath, "config.json"; got != want {
		t.Fatalf("unexpected first target path %q", got)
	}
	if got, want := result.ObjectSource.Files[1].ETag, `"etag-model"`; got != want {
		t.Fatalf("unexpected second etag %q", got)
	}
}

func TestFetchRemoteModelHuggingFaceGGUFPlansDirectObjectSourceWithoutMirror(t *testing.T) {
	previousInfoFetcher := fetchHuggingFaceInfoFunc
	previousBaseURL := huggingFaceBaseURL
	t.Cleanup(func() {
		fetchHuggingFaceInfoFunc = previousInfoFetcher
		huggingFaceBaseURL = previousBaseURL
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got, want := request.Header.Get("Authorization"), "Bearer hf-token"; got != want {
			t.Fatalf("unexpected authorization header %q", got)
		}
		switch {
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/deepseek-r1-8b-q4_k_m.gguf":
			writer.Header().Set("Content-Length", "42")
			writer.Header().Set("ETag", `"etag-gguf"`)
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	fetchHuggingFaceInfoFunc = func(context.Context, string, string, string) (HuggingFaceInfo, error) {
		return HuggingFaceInfo{
			ID:          "owner/model",
			SHA:         "deadbeef",
			PipelineTag: "text-generation",
			License:     "apache-2.0",
			Files:       []string{"deepseek-r1-8b-q4_k_m.gguf"},
		}, nil
	}
	huggingFaceBaseURL = server.URL

	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:                      "https://huggingface.co/owner/model?revision=main",
		HFToken:                  "hf-token",
		SkipLocalMaterialization: true,
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}

	if got := result.ModelDir; got != "" {
		t.Fatalf("expected no local model dir, got %q", got)
	}
	if result.ObjectSource == nil {
		t.Fatal("expected direct object source")
	}
	if result.ProfileSummary == nil {
		t.Fatal("expected remote profile summary")
	}
	if got, want := result.ProfileSummary.ModelFileName, "deepseek-r1-8b-q4_k_m.gguf"; got != want {
		t.Fatalf("unexpected model file name %q", got)
	}
	if got, want := result.ProfileSummary.ModelSizeBytes, int64(42); got != want {
		t.Fatalf("unexpected model size %d", got)
	}
}

func TestHuggingFaceObjectSourceReaderUsesIdentityEncodingAndToleratesMissingContentLengthOnGet(t *testing.T) {
	previousInfoFetcher := fetchHuggingFaceInfoFunc
	previousBaseURL := huggingFaceBaseURL
	t.Cleanup(func() {
		fetchHuggingFaceInfoFunc = previousInfoFetcher
		huggingFaceBaseURL = previousBaseURL
	})

	configPayload := []byte(`{"architectures":["TinyModel"]}`)
	modelPayload := []byte("safetensors-body")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got, want := request.Header.Get("Authorization"), "Bearer hf-token"; got != want {
			t.Fatalf("unexpected authorization header %q", got)
		}
		if got, want := request.Header.Get("Accept-Encoding"), "identity"; got != want {
			t.Fatalf("unexpected Accept-Encoding header %q", got)
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/owner/model/resolve/deadbeef/config.json":
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("Content-Length", strconv.Itoa(len(configPayload)))
			writer.Header().Set("ETag", `"etag-config"`)
			_, _ = writer.Write(configPayload)
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/config.json":
			writer.Header().Set("Content-Length", strconv.Itoa(len(configPayload)))
			writer.Header().Set("ETag", `"etag-config"`)
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/model.safetensors":
			writer.Header().Set("Content-Length", strconv.Itoa(len(modelPayload)))
			writer.Header().Set("ETag", `"etag-model"`)
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodGet && request.URL.Path == "/owner/model/resolve/deadbeef/model.safetensors":
			writer.Header().Set("ETag", `"etag-model"`)
			_, _ = writer.Write(modelPayload[:4])
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			_, _ = writer.Write(modelPayload[4:])
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	fetchHuggingFaceInfoFunc = func(context.Context, string, string, string) (HuggingFaceInfo, error) {
		return HuggingFaceInfo{
			ID:          "owner/model",
			SHA:         "deadbeef",
			PipelineTag: "text-generation",
			License:     "apache-2.0",
			Files:       []string{"config.json", "model.safetensors"},
		}, nil
	}
	huggingFaceBaseURL = server.URL

	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:                      "https://huggingface.co/owner/model?revision=main",
		HFToken:                  "hf-token",
		SkipLocalMaterialization: true,
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}
	if result.ObjectSource == nil {
		t.Fatal("expected direct object source")
	}

	object, err := result.ObjectSource.Reader.OpenRead(t.Context(), result.ObjectSource.Files[1].SourcePath)
	if err != nil {
		t.Fatalf("OpenRead() error = %v", err)
	}
	defer object.Body.Close()

	payload, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if got, want := string(payload), string(modelPayload); got != want {
		t.Fatalf("unexpected object payload %q, want %q", got, want)
	}
	if got := object.SizeBytes; got != 0 {
		t.Fatalf("OpenRead().SizeBytes = %d, want 0 when GET omits Content-Length", got)
	}
}

func TestHuggingFaceObjectSourceReaderOpenReadRangeUsesContentRangeLength(t *testing.T) {
	reader := huggingFaceHTTPObjectReader{
		httpClient: http.DefaultClient,
		token:      "hf-token",
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got, want := request.Header.Get("Authorization"), "Bearer hf-token"; got != want {
			t.Fatalf("unexpected authorization header %q", got)
		}
		if got, want := request.Header.Get("Accept-Encoding"), "identity"; got != want {
			t.Fatalf("unexpected Accept-Encoding header %q", got)
		}
		if got, want := request.Header.Get("Range"), "bytes=4-7"; got != want {
			t.Fatalf("unexpected Range header %q", got)
		}
		writer.Header().Set("Content-Range", "bytes 4-7/16")
		writer.Header().Set("ETag", `"etag-range"`)
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write([]byte("4567"))
	}))
	defer server.Close()

	object, err := reader.OpenReadRange(t.Context(), server.URL+"/owner/model/resolve/deadbeef/model.safetensors", 4, 4)
	if err != nil {
		t.Fatalf("OpenReadRange() error = %v", err)
	}
	defer object.Body.Close()

	payload, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if got, want := string(payload), "4567"; got != want {
		t.Fatalf("unexpected range payload %q, want %q", got, want)
	}
	if got, want := object.SizeBytes, int64(4); got != want {
		t.Fatalf("OpenReadRange().SizeBytes = %d, want %d", got, want)
	}
}
