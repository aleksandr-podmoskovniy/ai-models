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

package sourcemirror

import (
	"strings"
	"testing"
)

func TestSnapshotLocatorValidate(t *testing.T) {
	t.Parallel()

	valid := SnapshotLocator{
		Provider: "huggingface",
		Subject:  "google/gemma-4-E2B-it",
		Revision: "b4a601102c3d45e2b7b50e2057a6d5ec8ed4adcf",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	for _, locator := range []SnapshotLocator{
		{Provider: "", Subject: "google/gemma-4-E2B-it", Revision: "sha"},
		{Provider: ".", Subject: "google/gemma-4-E2B-it", Revision: "sha"},
		{Provider: "huggingface", Subject: "../gemma", Revision: "sha"},
		{Provider: "huggingface", Subject: "google//gemma", Revision: "sha"},
		{Provider: "huggingface", Subject: "google/gemma", Revision: ".."},
	} {
		if err := locator.Validate(); err == nil {
			t.Fatalf("Validate() unexpectedly accepted locator %#v", locator)
		}
	}
}

func TestSnapshotManifestValidate(t *testing.T) {
	t.Parallel()

	manifest := SnapshotManifest{
		Locator: SnapshotLocator{
			Provider: "huggingface",
			Subject:  "google/gemma-4-E2B-it",
			Revision: "sha",
		},
		Files: []SnapshotFile{
			{Path: "config.json", SizeBytes: 123},
			{Path: "model.safetensors", SizeBytes: 456},
		},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	manifest.Files = append(manifest.Files, SnapshotFile{Path: "model.safetensors"})
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "duplicates path") {
		t.Fatalf("Validate() error = %v, want duplicate path error", err)
	}
}

func TestSnapshotStateValidate(t *testing.T) {
	t.Parallel()

	state := SnapshotState{
		Locator: SnapshotLocator{
			Provider: "huggingface",
			Subject:  "google/gemma-4-E2B-it",
			Revision: "sha",
		},
		Phase: SnapshotPhaseDownloading,
		Files: []SnapshotFileState{
			{Path: "model.safetensors", Phase: SnapshotPhaseDownloading, BytesConfirmed: 1024},
		},
	}
	if err := state.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	state.Files[0].Path = "../evil"
	if err := state.Validate(); err == nil {
		t.Fatal("Validate() unexpectedly accepted escaping path")
	}
}

func TestSnapshotPrefix(t *testing.T) {
	t.Parallel()

	prefix := SnapshotPrefix("raw/1111-2222/source-url/.mirror", SnapshotLocator{
		Provider: "huggingface",
		Subject:  "google/gemma-4-E2B-it",
		Revision: "deadbeef",
	})
	if got, want := prefix, "raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef"; got != want {
		t.Fatalf("unexpected snapshot prefix %q", got)
	}
}

func TestSnapshotFileObjectKey(t *testing.T) {
	t.Parallel()

	key := SnapshotFileObjectKey("raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef", "tokenizer/config.json")
	if got, want := key, "raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef/files/tokenizer/config.json"; got != want {
		t.Fatalf("unexpected snapshot file object key %q", got)
	}
}
