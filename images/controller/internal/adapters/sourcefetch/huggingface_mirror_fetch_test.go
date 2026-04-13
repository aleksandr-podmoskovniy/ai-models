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
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
)

func TestFetchRemoteModelHuggingFaceUsesSourceMirror(t *testing.T) {
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

	mirrorStore := &fakeSourceMirrorStore{}
	mirrorClient := newFakeMirrorUploadStaging(t)

	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:       "https://huggingface.co/owner/model?revision=main",
		Workspace: t.TempDir(),
		HFToken:   "hf-token",
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

	if got, want := len(result.StagedObjects), 0; got != want {
		t.Fatalf("unexpected staged object count %d", got)
	}
	if result.SourceMirror == nil {
		t.Fatal("expected source mirror snapshot")
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
	if payload, err := os.ReadFile(filepath.Join(result.ModelDir, "model.safetensors")); err != nil {
		t.Fatalf("ReadFile(model.safetensors) error = %v", err)
	} else if got, want := string(payload), "tensor-payload"; got != want {
		t.Fatalf("unexpected materialized payload %q", got)
	}
	if got, want := result.InputFormat, modelsv1alpha1.ModelInputFormatSafetensors; got != want {
		t.Fatalf("unexpected input format %q", got)
	}
}
