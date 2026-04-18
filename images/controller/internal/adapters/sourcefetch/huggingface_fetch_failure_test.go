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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestFetchRemoteModelHuggingFaceFailsWhenRemoteProfileSummaryCannotBeResolved(t *testing.T) {
	previousInfoFetcher := fetchHuggingFaceInfoFunc
	previousProfileSummaryFetcher := fetchHuggingFaceProfileSummaryFunc
	t.Cleanup(func() {
		fetchHuggingFaceInfoFunc = previousInfoFetcher
		fetchHuggingFaceProfileSummaryFunc = previousProfileSummaryFetcher
	})

	fetchHuggingFaceInfoFunc = func(context.Context, string, string, string) (HuggingFaceInfo, error) {
		return HuggingFaceInfo{
			ID:          "owner/model",
			SHA:         "deadbeef",
			PipelineTag: "text-generation",
			License:     "apache-2.0",
			Files:       []string{"config.json", "model.safetensors"},
		}, nil
	}
	fetchHuggingFaceProfileSummaryFunc = func(context.Context, RemoteOptions, string, string, modelsv1alpha1.ModelInputFormat, []string) (*RemoteProfileSummary, error) {
		return nil, errors.New("summary unavailable")
	}

	_, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:                      "https://huggingface.co/owner/model?revision=main",
		HFToken:                  "hf-token",
		SkipLocalMaterialization: true,
	})
	if err == nil {
		t.Fatal("expected missing remote profile summary to fail")
	}
}

func TestFetchRemoteModelHuggingFaceFailsWhenDirectObjectSourcePlanningFails(t *testing.T) {
	previousInfoFetcher := fetchHuggingFaceInfoFunc
	previousBaseURL := huggingFaceBaseURL
	previousProfileSummaryFetcher := fetchHuggingFaceProfileSummaryFunc
	t.Cleanup(func() {
		fetchHuggingFaceInfoFunc = previousInfoFetcher
		huggingFaceBaseURL = previousBaseURL
		fetchHuggingFaceProfileSummaryFunc = previousProfileSummaryFetcher
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
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
	fetchHuggingFaceProfileSummaryFunc = func(context.Context, RemoteOptions, string, string, modelsv1alpha1.ModelInputFormat, []string) (*RemoteProfileSummary, error) {
		return &RemoteProfileSummary{
			ConfigPayload: []byte(`{"architectures":["LlamaForCausalLM"]}`),
			WeightBytes:   14,
		}, nil
	}
	huggingFaceBaseURL = server.URL

	_, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:                      "https://huggingface.co/owner/model?revision=main",
		HFToken:                  "hf-token",
		SkipLocalMaterialization: true,
	})
	if err == nil {
		t.Fatal("expected direct object-source planning failure to fail publish source preparation")
	}
}
