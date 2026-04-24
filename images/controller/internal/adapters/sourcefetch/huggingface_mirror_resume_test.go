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
	"time"

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func TestMirrorHuggingFaceSnapshotFilesResumesMultipartUpload(t *testing.T) {
	var rangeHeader string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		rangeHeader = request.Header.Get("Range")
		if got, want := rangeHeader, "bytes=3-"; got != want {
			t.Fatalf("unexpected range header %q", got)
		}
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write([]byte("defghi"))
	}))
	defer server.Close()
	withHuggingFaceBaseURL(t, server.URL)

	mirrorClient := newFakeMirrorUploadStaging(t)
	snapshot := newTestSourceMirrorSnapshot()
	mirrorKey := sourcemirrorports.SnapshotFileObjectKey(snapshot.CleanupPrefix, "model.safetensors")
	mirrorClient.seedMultipartUpload("upload-1")
	mirrorClient.seedUploadedPart("upload-1", 1, []byte("abc"))
	store := &fakeSourceMirrorStore{
		state: sourcemirrorports.SnapshotState{
			Locator: snapshot.Locator,
			Phase:   sourcemirrorports.SnapshotPhaseDownloading,
			Files: []sourcemirrorports.SnapshotFileState{
				{
					Path:              "model.safetensors",
					Phase:             sourcemirrorports.SnapshotPhaseDownloading,
					BytesConfirmed:    3,
					MultipartUploadID: "upload-1",
					CompletedParts: []uploadstagingports.CompletedPart{
						{PartNumber: 1, ETag: `"etag-1"`},
					},
					UpdatedAt: time.Now().UTC(),
				},
			},
		},
	}

	err := mirrorHuggingFaceSnapshotFiles(t.Context(), &SourceMirrorOptions{
		Bucket:     "artifacts",
		Client:     mirrorClient,
		Store:      store,
		BasePrefix: testSourceMirrorBasePrefix,
	}, testHuggingFaceSubject, testHuggingFaceRevision, testHuggingFaceToken, []string{"model.safetensors"}, snapshot)
	if err != nil {
		t.Fatalf("mirrorHuggingFaceSnapshotFiles() error = %v", err)
	}

	if got, want := string(mirrorClient.objects["artifacts/"+mirrorKey]), "abcdefghi"; got != want {
		t.Fatalf("unexpected mirrored object payload %q", got)
	}
	if got, want := store.state.Phase, sourcemirrorports.SnapshotPhaseCompleted; got != want {
		t.Fatalf("unexpected mirror state phase %q", got)
	}
	if got, want := store.state.Files[0].BytesConfirmed, int64(9); got != want {
		t.Fatalf("unexpected mirrored byte count %d", got)
	}
	if got, want := snapshot.SizeBytes, int64(9); got != want {
		t.Fatalf("unexpected snapshot size %d", got)
	}
}
