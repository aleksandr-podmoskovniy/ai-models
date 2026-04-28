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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestFetchHuggingFaceProfileSummarySafetensors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireHuggingFaceAuth(t, request)
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/owner/model/resolve/deadbeef/config.json":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"architectures":["LlamaForCausalLM"],"torch_dtype":"bfloat16"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/owner/model/resolve/deadbeef/tokenizer_config.json":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"chat_template":"{%- if tools %}{{ tools }}{%- endif %}"}`))
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/model-00001-of-00002.safetensors":
			writer.Header().Set("Content-Length", "11")
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/model-00002-of-00002.safetensors":
			writer.Header().Set("Content-Length", "13")
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	withHuggingFaceBaseURL(t, server.URL)
	summary, err := fetchHuggingFaceProfileSummary(t.Context(), RemoteOptions{
		HFToken: testHuggingFaceToken,
	}, "owner/model", "deadbeef", modelsv1alpha1.ModelInputFormatSafetensors, []string{
		"config.json",
		"tokenizer_config.json",
		"model-00001-of-00002.safetensors",
		"model-00002-of-00002.safetensors",
	})
	if err != nil {
		t.Fatalf("fetchHuggingFaceProfileSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatal("expected remote profile summary")
	}
	if got, want := summary.WeightBytes, int64(24); got != want {
		t.Fatalf("unexpected weight bytes %d", got)
	}
	if got, want := summary.LargestWeightFileBytes, int64(13); got != want {
		t.Fatalf("unexpected largest weight bytes %d", got)
	}
	if got, want := summary.WeightFileCount, int64(2); got != want {
		t.Fatalf("unexpected weight file count %d", got)
	}
	if got, want := string(summary.ConfigPayload), `{"architectures":["LlamaForCausalLM"],"torch_dtype":"bfloat16"}`; got != want {
		t.Fatalf("unexpected config payload %q", got)
	}
	if got, want := string(summary.TokenizerConfigPayload), `{"chat_template":"{%- if tools %}{{ tools }}{%- endif %}"}`; got != want {
		t.Fatalf("unexpected tokenizer config payload %q", got)
	}
}

func TestFetchHuggingFaceProfileSummaryDiffusers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireHuggingFaceAuth(t, request)
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/owner/model/resolve/deadbeef/model_index.json":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"_class_name":"StableDiffusionPipeline"}`))
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/text_encoder/pytorch_model.bin":
			writer.Header().Set("Content-Length", "17")
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodHead && request.URL.Path == "/owner/model/resolve/deadbeef/unet/diffusion_pytorch_model.bin":
			writer.Header().Set("Content-Length", "29")
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	withHuggingFaceBaseURL(t, server.URL)
	summary, err := fetchHuggingFaceProfileSummary(t.Context(), RemoteOptions{
		HFToken: testHuggingFaceToken,
	}, "owner/model", "deadbeef", modelsv1alpha1.ModelInputFormatDiffusers, []string{
		"model_index.json",
		"scheduler/scheduler_config.json",
		"text_encoder/config.json",
		"text_encoder/pytorch_model.bin",
		"unet/config.json",
		"unet/diffusion_pytorch_model.bin",
	})
	if err != nil {
		t.Fatalf("fetchHuggingFaceProfileSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatal("expected remote profile summary")
	}
	if got, want := string(summary.ModelIndexPayload), `{"_class_name":"StableDiffusionPipeline"}`; got != want {
		t.Fatalf("unexpected model index payload %q", got)
	}
	if got, want := summary.WeightBytes, int64(46); got != want {
		t.Fatalf("unexpected weight bytes %d", got)
	}
	if got, want := summary.LargestWeightFileBytes, int64(29); got != want {
		t.Fatalf("unexpected largest weight bytes %d", got)
	}
}

func TestFetchHuggingFaceProfileSummaryGGUF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireHuggingFaceAuth(t, request)
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

	withHuggingFaceBaseURL(t, server.URL)
	summary, err := fetchHuggingFaceProfileSummary(t.Context(), RemoteOptions{
		HFToken: testHuggingFaceToken,
	}, "owner/model", "deadbeef", modelsv1alpha1.ModelInputFormatGGUF, []string{
		"deepseek-r1-8b-q4_k_m.gguf",
	})
	if err != nil {
		t.Fatalf("fetchHuggingFaceProfileSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatal("expected remote profile summary")
	}
	if got, want := summary.ModelFileName, "deepseek-r1-8b-q4_k_m.gguf"; got != want {
		t.Fatalf("unexpected model file name %q", got)
	}
	if got, want := summary.ModelSizeBytes, int64(42); got != want {
		t.Fatalf("unexpected model size %d", got)
	}
}
