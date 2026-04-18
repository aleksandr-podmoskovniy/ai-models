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
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
)

func TestFetchRemoteModelHuggingFaceUsesSourceMirrorStreamingPublish(t *testing.T) {
	previousInfoFetcher := fetchHuggingFaceInfoFunc
	previousBaseURL := huggingFaceBaseURL
	previousProfileSummaryFetcher := fetchHuggingFaceProfileSummaryFunc
	t.Cleanup(func() {
		fetchHuggingFaceInfoFunc = previousInfoFetcher
		huggingFaceBaseURL = previousBaseURL
		fetchHuggingFaceProfileSummaryFunc = previousProfileSummaryFetcher
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got, want := request.Header.Get("Authorization"), "Bearer hf-token"; got != want {
			t.Fatalf("unexpected authorization header %q", got)
		}
		switch request.URL.Path {
		case "/owner/model/resolve/deadbeef/config.json":
			_, _ = writer.Write([]byte(`{"architectures":["LlamaForCausalLM"]}`))
		case "/owner/model/resolve/deadbeef/model.safetensors":
			_, _ = writer.Write([]byte("tensor-payload"))
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
	fetchHuggingFaceProfileSummaryFunc = func(context.Context, RemoteOptions, string, string, modelsv1alpha1.ModelInputFormat, []string) (*RemoteProfileSummary, error) {
		return &RemoteProfileSummary{
			ConfigPayload: []byte(`{"architectures":["LlamaForCausalLM"]}`),
			WeightBytes:   14,
		}, nil
	}

	mirrorStore := &fakeSourceMirrorStore{}
	mirrorClient := newFakeMirrorUploadStaging(t)
	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:                      "https://huggingface.co/owner/model?revision=main",
		HFToken:                  "hf-token",
		SkipLocalMaterialization: true,
		SourceMirror: &SourceMirrorOptions{
			Bucket:     "artifacts",
			Client:     mirrorClient,
			Store:      mirrorStore,
			BasePrefix: "raw/1111-2222/source-url/.mirror",
		},
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}

	if result.SourceMirror == nil {
		t.Fatal("expected source mirror snapshot")
	}
	if result.ProfileSummary == nil {
		t.Fatal("expected remote profile summary")
	}
	if got := result.ModelDir; got != "" {
		t.Fatalf("expected no local model dir, got %q", got)
	}
	if got, want := result.SourceMirror.ObjectCount, int64(2); got != want {
		t.Fatalf("unexpected source mirror object count %d", got)
	}
	if got, want := string(mirrorClient.objects["artifacts/raw/1111-2222/source-url/.mirror/huggingface/owner/model/deadbeef/files/model.safetensors"]), "tensor-payload"; got != want {
		t.Fatalf("unexpected mirrored model payload %q", got)
	}
	if got, want := mirrorStore.state.Phase, sourcemirrorports.SnapshotPhaseCompleted; got != want {
		t.Fatalf("unexpected mirror phase %q", got)
	}
}

func TestFetchRemoteModelHuggingFaceSourceMirrorFailsWhenRemoteProfileSummaryCannotBeResolved(t *testing.T) {
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

	mirrorStore := &fakeSourceMirrorStore{}
	mirrorClient := newFakeMirrorUploadStaging(t)
	_, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:                      "https://huggingface.co/owner/model?revision=main",
		HFToken:                  "hf-token",
		SkipLocalMaterialization: true,
		SourceMirror: &SourceMirrorOptions{
			Bucket:     "artifacts",
			Client:     mirrorClient,
			Store:      mirrorStore,
			BasePrefix: "raw/1111-2222/source-url/.mirror",
		},
	})
	if err == nil {
		t.Fatal("expected missing remote profile summary to fail mirrored publish source planning")
	}
	if got := len(mirrorClient.objects); got != 0 {
		t.Fatalf("expected no mirrored objects on summary failure, got %d", got)
	}
}
