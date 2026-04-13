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

func TestMirrorHuggingFaceSnapshotFilesUsesCustomUploadHTTPClient(t *testing.T) {
	previousBaseURL := huggingFaceBaseURL
	t.Cleanup(func() { huggingFaceBaseURL = previousBaseURL })

	hfServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("tensor-payload"))
	}))
	defer hfServer.Close()
	huggingFaceBaseURL = hfServer.URL

	mirrorClient, uploadHTTPClient := newFakeMirrorUploadStagingTLS(t)
	snapshot := &SourceMirrorSnapshot{
		Locator: sourcemirrorports.SnapshotLocator{
			Provider: "huggingface",
			Subject:  "owner/model",
			Revision: "deadbeef",
		},
		CleanupPrefix: "raw/1111-2222/source-url/.mirror/huggingface/owner/model/deadbeef",
	}
	store := &fakeSourceMirrorStore{}

	err := mirrorHuggingFaceSnapshotFiles(t.Context(), &SourceMirrorOptions{
		Bucket:           "artifacts",
		Client:           mirrorClient,
		UploadHTTPClient: uploadHTTPClient,
		Store:            store,
		BasePrefix:       "raw/1111-2222/source-url/.mirror",
	}, "owner/model", "deadbeef", "hf-token", []string{"model.safetensors"}, snapshot)
	if err != nil {
		t.Fatalf("mirrorHuggingFaceSnapshotFiles() error = %v", err)
	}

	key := sourcemirrorports.SnapshotFileObjectKey(snapshot.CleanupPrefix, "model.safetensors")
	if got, want := string(mirrorClient.objects["artifacts/"+key]), "tensor-payload"; got != want {
		t.Fatalf("unexpected mirrored object payload %q", got)
	}
}
