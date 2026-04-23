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

package directupload

import (
	"strings"
	"testing"
)

func TestBlobDataObjectKeyIncludesDMCRRoot(t *testing.T) {
	t.Parallel()

	got, err := BlobDataObjectKey("/dmcr", "sha256:"+strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("BlobDataObjectKey() error = %v", err)
	}
	want := "dmcr/docker/registry/v2/blobs/sha256/aa/" + strings.Repeat("a", 64) + "/data"
	if got != want {
		t.Fatalf("BlobDataObjectKey() = %q, want %q", got, want)
	}
}

func TestRepositoryBlobLinkObjectKeyIncludesRepositoryPath(t *testing.T) {
	t.Parallel()

	got, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", "sha256:"+strings.Repeat("b", 64))
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	want := "dmcr/docker/registry/v2/repositories/ai-models/catalog/model/_layers/sha256/" + strings.Repeat("b", 64) + "/link"
	if got != want {
		t.Fatalf("RepositoryBlobLinkObjectKey() = %q, want %q", got, want)
	}
}

func TestStorageDriverPathForObjectKeyRemovesRootDirectory(t *testing.T) {
	t.Parallel()

	got := storageDriverPathForObjectKey("/dmcr", "dmcr/_ai_models/direct-upload/objects/session/data")
	want := "/_ai_models/direct-upload/objects/session/data"
	if got != want {
		t.Fatalf("storageDriverPathForObjectKey() = %q, want %q", got, want)
	}
}

func TestStorageDriverPathForObjectKeyKeepsRootlessObjectKeyAbsolute(t *testing.T) {
	t.Parallel()

	got := storageDriverPathForObjectKey("", "_ai_models/direct-upload/objects/session/data")
	want := "/_ai_models/direct-upload/objects/session/data"
	if got != want {
		t.Fatalf("storageDriverPathForObjectKey() = %q, want %q", got, want)
	}
}
