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

func TestMirrorHuggingFaceSnapshotFilesUsesCustomUploadHTTPClient(t *testing.T) {
	hfServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("tensor-payload"))
	}))
	defer hfServer.Close()
	withHuggingFaceBaseURL(t, hfServer.URL)

	mirrorClient, uploadHTTPClient := newFakeMirrorUploadStagingTLS(t)
	snapshot := newTestSourceMirrorSnapshot()
	store := &fakeSourceMirrorStore{}

	err := mirrorHuggingFaceSnapshotFiles(t.Context(), &SourceMirrorOptions{
		Bucket:           "artifacts",
		Client:           mirrorClient,
		UploadHTTPClient: uploadHTTPClient,
		Store:            store,
		BasePrefix:       testSourceMirrorBasePrefix,
	}, testHuggingFaceSubject, testHuggingFaceRevision, testHuggingFaceToken, []string{"model.safetensors"}, snapshot)
	if err != nil {
		t.Fatalf("mirrorHuggingFaceSnapshotFiles() error = %v", err)
	}

	key := snapshot.CleanupPrefix + "/files/model.safetensors"
	if got, want := string(mirrorClient.objects["artifacts/"+key]), "tensor-payload"; got != want {
		t.Fatalf("unexpected mirrored object payload %q", got)
	}
}
