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

package objectstore

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type fakeObjectStore struct {
	objects map[string][]byte
}

func (f *fakeObjectStore) Upload(_ context.Context, input uploadstagingports.UploadInput) error {
	if f.objects == nil {
		f.objects = make(map[string][]byte)
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return err
	}
	f.objects[input.Bucket+"/"+input.Key] = body
	return nil
}

func (f *fakeObjectStore) Download(_ context.Context, input uploadstagingports.DownloadInput) error {
	body, ok := f.objects[input.Bucket+"/"+input.Key]
	if !ok {
		return os.ErrNotExist
	}
	return os.WriteFile(input.DestinationPath, body, 0o644)
}

func (f *fakeObjectStore) DeletePrefix(_ context.Context, input uploadstagingports.DeletePrefixInput) error {
	prefix := input.Bucket + "/" + input.Prefix
	for key := range f.objects {
		if strings.HasPrefix(key, prefix) {
			delete(f.objects, key)
		}
	}
	return nil
}

func TestAdapterManifestAndStateRoundTrip(t *testing.T) {
	t.Parallel()

	store := &fakeObjectStore{}
	adapter := &Adapter{
		Uploader:      store,
		Downloader:    store,
		PrefixRemover: store,
		Bucket:        "artifacts",
		BasePrefix:    "raw/source-mirror",
	}
	manifest := sourcemirrorports.SnapshotManifest{
		Locator: sourcemirrorports.SnapshotLocator{
			Provider: "huggingface",
			Subject:  "google/gemma-4-E2B-it",
			Revision: "sha",
		},
		Files: []sourcemirrorports.SnapshotFile{
			{Path: "config.json", SizeBytes: 101},
			{Path: "model.safetensors", SizeBytes: 102},
		},
		CreatedAt: time.Unix(1700000000, 0).UTC(),
	}
	if err := adapter.SaveManifest(context.Background(), manifest); err != nil {
		t.Fatalf("SaveManifest() error = %v", err)
	}

	loadedManifest, err := adapter.LoadManifest(context.Background(), manifest.Locator)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if got, want := loadedManifest.Locator, manifest.Locator; got != want {
		t.Fatalf("unexpected manifest locator %#v", got)
	}
	if got, want := len(loadedManifest.Files), 2; got != want {
		t.Fatalf("unexpected manifest file count %d", got)
	}

	state := sourcemirrorports.SnapshotState{
		Locator:   manifest.Locator,
		Phase:     sourcemirrorports.SnapshotPhaseDownloading,
		UpdatedAt: time.Unix(1700000100, 0).UTC(),
		Files: []sourcemirrorports.SnapshotFileState{
			{
				Path:              "model.safetensors",
				Phase:             sourcemirrorports.SnapshotPhaseDownloading,
				BytesConfirmed:    64,
				MultipartUploadID: "upload-1",
				CompletedParts: []uploadstagingports.CompletedPart{
					{PartNumber: 1, ETag: "etag-1"},
				},
			},
		},
	}
	if err := adapter.SaveState(context.Background(), state); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	loadedState, err := adapter.LoadState(context.Background(), state.Locator)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got, want := loadedState.Phase, sourcemirrorports.SnapshotPhaseDownloading; got != want {
		t.Fatalf("unexpected state phase %q", got)
	}
	if got, want := loadedState.Files[0].MultipartUploadID, "upload-1"; got != want {
		t.Fatalf("unexpected upload id %q", got)
	}

	if err := adapter.DeleteSnapshot(context.Background(), manifest.Locator); err != nil {
		t.Fatalf("DeleteSnapshot() error = %v", err)
	}
	if _, err := adapter.LoadManifest(context.Background(), manifest.Locator); !errors.Is(err, sourcemirrorports.ErrSnapshotNotFound) {
		t.Fatalf("LoadManifest() error = %v, want ErrSnapshotNotFound", err)
	}
}

func TestAdapterSnapshotPrefix(t *testing.T) {
	t.Parallel()

	locator := sourcemirrorports.SnapshotLocator{
		Provider: "HuggingFace",
		Subject:  "google/gemma-4-E2B-it",
		Revision: "b4a60110",
	}
	if got, want := snapshotPrefix("raw/source-mirror", locator), "raw/source-mirror/huggingface/google/gemma-4-E2B-it/b4a60110"; got != want {
		t.Fatalf("unexpected snapshot prefix %q", got)
	}
}
