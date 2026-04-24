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

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
)

func TestFetchRemoteModelHuggingFaceUsesSourceMirrorStreamingPublish(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireHuggingFaceAuth(t, request)
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

	stubDefaultHuggingFaceInfo(t)
	withHuggingFaceBaseURL(t, server.URL)
	stubHuggingFaceProfileSummary(t, defaultHuggingFaceProfileSummary(), nil)

	mirrorStore := &fakeSourceMirrorStore{}
	mirrorClient := newFakeMirrorUploadStaging(t)
	result, err := fetchTestHuggingFaceRemote(t, newTestSourceMirrorOptions(mirrorClient, mirrorStore))
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
	stubDefaultHuggingFaceInfo(t)
	stubUnavailableHuggingFaceProfileSummary(t)

	mirrorStore := &fakeSourceMirrorStore{}
	mirrorClient := newFakeMirrorUploadStaging(t)
	_, err := fetchTestHuggingFaceRemote(t, newTestSourceMirrorOptions(mirrorClient, mirrorStore))
	if err == nil {
		t.Fatal("expected missing remote profile summary to fail mirrored publish source planning")
	}
	if got := len(mirrorClient.objects); got != 0 {
		t.Fatalf("expected no mirrored objects on summary failure, got %d", got)
	}
}
