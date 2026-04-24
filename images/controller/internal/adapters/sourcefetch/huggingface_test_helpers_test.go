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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
)

const (
	testHuggingFaceSubject      = "owner/model"
	testHuggingFaceRevision     = "deadbeef"
	testHuggingFaceToken        = "hf-token"
	testHuggingFaceRemoteURL    = "https://huggingface.co/owner/model?revision=main"
	testSourceMirrorBasePrefix  = "raw/1111-2222/source-url/.mirror"
	testSourceMirrorCleanPrefix = testSourceMirrorBasePrefix + "/huggingface/owner/model/deadbeef"
)

func withHuggingFaceBaseURL(t *testing.T, baseURL string) {
	t.Helper()
	previousBaseURL := huggingFaceBaseURL
	huggingFaceBaseURL = baseURL
	t.Cleanup(func() { huggingFaceBaseURL = previousBaseURL })
}

func stubDefaultHuggingFaceInfo(t *testing.T, files ...string) {
	t.Helper()
	stubHuggingFaceInfo(t, defaultHuggingFaceInfo(files...))
}

func stubHuggingFaceInfo(t *testing.T, info HuggingFaceInfo) {
	t.Helper()
	previousInfoFetcher := fetchHuggingFaceInfoFunc
	fetchHuggingFaceInfoFunc = func(context.Context, string, string, string) (HuggingFaceInfo, error) {
		return info, nil
	}
	t.Cleanup(func() { fetchHuggingFaceInfoFunc = previousInfoFetcher })
}

func defaultHuggingFaceInfo(files ...string) HuggingFaceInfo {
	if len(files) == 0 {
		files = []string{"config.json", "model.safetensors"}
	}
	return HuggingFaceInfo{
		ID:          testHuggingFaceSubject,
		SHA:         testHuggingFaceRevision,
		PipelineTag: "text-generation",
		License:     "apache-2.0",
		Files:       files,
	}
}

func stubHuggingFaceProfileSummary(t *testing.T, summary *RemoteProfileSummary, err error) {
	t.Helper()
	previousProfileSummaryFetcher := fetchHuggingFaceProfileSummaryFunc
	fetchHuggingFaceProfileSummaryFunc = func(context.Context, RemoteOptions, string, string, modelsv1alpha1.ModelInputFormat, []string) (*RemoteProfileSummary, error) {
		if err != nil {
			return nil, err
		}
		return summary, nil
	}
	t.Cleanup(func() { fetchHuggingFaceProfileSummaryFunc = previousProfileSummaryFetcher })
}

func stubUnavailableHuggingFaceProfileSummary(t *testing.T) {
	t.Helper()
	stubHuggingFaceProfileSummary(t, nil, errors.New("summary unavailable"))
}

func fetchTestHuggingFaceRemote(t *testing.T, mirror *SourceMirrorOptions) (RemoteResult, error) {
	t.Helper()
	return FetchRemoteModel(t.Context(), RemoteOptions{
		URL:                      testHuggingFaceRemoteURL,
		HFToken:                  testHuggingFaceToken,
		SkipLocalMaterialization: true,
		SourceMirror:             mirror,
	})
}

func newTestSourceMirrorOptions(client SourceMirrorTransportClient, store sourcemirrorports.Store) *SourceMirrorOptions {
	return &SourceMirrorOptions{
		Bucket:     "artifacts",
		Client:     client,
		Store:      store,
		BasePrefix: testSourceMirrorBasePrefix,
	}
}

func defaultHuggingFaceProfileSummary() *RemoteProfileSummary {
	return &RemoteProfileSummary{
		ConfigPayload: []byte(`{"architectures":["LlamaForCausalLM"]}`),
		WeightBytes:   14,
	}
}

func requireHuggingFaceAuth(t *testing.T, request *http.Request) {
	t.Helper()
	if got, want := request.Header.Get("Authorization"), "Bearer "+testHuggingFaceToken; got != want {
		t.Fatalf("unexpected authorization header %q", got)
	}
}

func newTestSourceMirrorSnapshot() *SourceMirrorSnapshot {
	return &SourceMirrorSnapshot{
		Locator: sourcemirrorports.SnapshotLocator{
			Provider: "huggingface",
			Subject:  testHuggingFaceSubject,
			Revision: testHuggingFaceRevision,
		},
		CleanupPrefix: testSourceMirrorCleanPrefix,
	}
}
