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
	"slices"
	"testing"
)

func assertReaderCalls(t *testing.T, backend *fakeBackend, want int) {
	t.Helper()
	if got := backend.readerCalls; got != want {
		t.Fatalf("Reader() call count = %d, want %d", got, want)
	}
}

func assertObjectExists(t *testing.T, backend *fakeBackend, objectKey string) {
	t.Helper()
	if _, exists := backend.objects[objectKey]; !exists {
		t.Fatalf("object %q does not exist", objectKey)
	}
}

func assertObjectMissing(t *testing.T, backend *fakeBackend, objectKey string) {
	t.Helper()
	if _, exists := backend.objects[objectKey]; exists {
		t.Fatalf("object %q exists", objectKey)
	}
}

func assertDeleted(t *testing.T, backend *fakeBackend, objectKey string) {
	t.Helper()
	if !slices.Contains(backend.deleted, objectKey) {
		t.Fatalf("object %q was not deleted, deleted = %#v", objectKey, backend.deleted)
	}
}

func assertNotDeleted(t *testing.T, backend *fakeBackend, objectKey string) {
	t.Helper()
	if slices.Contains(backend.deleted, objectKey) {
		t.Fatalf("object %q was deleted, deleted = %#v", objectKey, backend.deleted)
	}
}

func mustBlobDataKey(t *testing.T, digest string) string {
	t.Helper()
	blobKey, err := BlobDataObjectKey("/dmcr", digest)
	if err != nil {
		t.Fatalf("BlobDataObjectKey() error = %v", err)
	}
	return blobKey
}

func mustRepositoryLinkKey(t *testing.T, repository, digest string) string {
	t.Helper()
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", repository, digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	return linkKey
}

func assertLinkPayload(t *testing.T, backend *fakeBackend, repository, digest string) {
	t.Helper()
	linkKey := mustRepositoryLinkKey(t, repository, digest)
	if got, want := string(backend.objects[linkKey]), digest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
}
