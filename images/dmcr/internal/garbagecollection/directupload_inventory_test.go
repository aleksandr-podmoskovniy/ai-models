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

package garbagecollection

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestDiscoverDirectUploadInventoryReturnsOnlyOldUnreferencedPrefixes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	protectedDigest := "sha256:" + strings.Repeat("a", 64)
	protectedMetadata, err := sealedblob.Marshal(sealedblob.Metadata{
		Version:      sealedblob.MetadataVersion,
		Digest:       protectedDigest,
		PhysicalPath: "/_ai_models/direct-upload/objects/session-live/data",
		SizeBytes:    42,
	})
	if err != nil {
		t.Fatalf("sealedblob.Marshal() error = %v", err)
	}

	store := newFakePrefixStore(
		fakePrefixObject{
			key:          "dmcr/_ai_models/direct-upload/objects/session-live/data",
			lastModified: now.Add(-48 * time.Hour),
		},
		fakePrefixObject{
			key:          "dmcr/_ai_models/direct-upload/objects/session-stale/data",
			lastModified: now.Add(-48 * time.Hour),
		},
		fakePrefixObject{
			key:          "dmcr/_ai_models/direct-upload/objects/session-fresh/data",
			lastModified: now.Add(-2 * time.Hour),
		},
		fakePrefixObject{
			key:     "dmcr/docker/registry/v2/blobs/sha256/aa/" + strings.Repeat("a", 64) + "/data.dmcr-sealed",
			payload: protectedMetadata,
		},
	)

	inventory, err := discoverDirectUploadInventory(
		context.Background(),
		store,
		"dmcr",
		now,
		defaultDirectUploadOrphanStaleAge,
	)
	if err != nil {
		t.Fatalf("discoverDirectUploadInventory() error = %v", err)
	}

	if got, want := inventory.StoredPrefixCount, 3; got != want {
		t.Fatalf("stored prefix count = %d, want %d", got, want)
	}
	if got, want := inventory.ReferencedPrefixCount, 1; got != want {
		t.Fatalf("referenced prefix count = %d, want %d", got, want)
	}
	if got, want := len(inventory.StalePrefixes), 1; got != want {
		t.Fatalf("stale prefix count = %d, want %d", got, want)
	}
	if got, want := inventory.StalePrefixes[0].Prefix, "dmcr/_ai_models/direct-upload/objects/session-stale"; got != want {
		t.Fatalf("stale prefix = %q, want %q", got, want)
	}
}

func TestDiscoverDirectUploadInventoryFailsClosedOnBrokenMetadata(t *testing.T) {
	t.Parallel()

	store := newFakePrefixStore(
		fakePrefixObject{
			key:          "dmcr/_ai_models/direct-upload/objects/session-a/data",
			lastModified: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		},
		fakePrefixObject{
			key:     "dmcr/docker/registry/v2/blobs/sha256/aa/" + strings.Repeat("a", 64) + "/data.dmcr-sealed",
			payload: []byte("not-json"),
		},
	)

	_, err := discoverDirectUploadInventory(
		context.Background(),
		store,
		"dmcr",
		time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC),
		defaultDirectUploadOrphanStaleAge,
	)
	if err == nil {
		t.Fatal("discoverDirectUploadInventory() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "decode sealed blob metadata") {
		t.Fatalf("error = %v, want decode sealed blob metadata", err)
	}
}

func TestDeleteStalePrefixesDeletesDirectUploadPrefixes(t *testing.T) {
	t.Parallel()

	store := newFakePrefixStore()
	report := Report{
		StaleDirectUploadPrefixes: []PrefixInventoryEntry{
			{Prefix: "dmcr/_ai_models/direct-upload/objects/session-stale"},
		},
	}

	if err := deleteStalePrefixes(context.Background(), store, report); err != nil {
		t.Fatalf("deleteStalePrefixes() error = %v", err)
	}

	if got, want := store.deletedPrefixes, []string{"dmcr/_ai_models/direct-upload/objects/session-stale/"}; !equalStringSlices(got, want) {
		t.Fatalf("deleted prefixes = %#v, want %#v", got, want)
	}
}

func TestDeleteStalePrefixesBoundsDirectUploadDeletionToOneSessionPrefix(t *testing.T) {
	t.Parallel()

	store := newDeletingFakePrefixStore(
		fakePrefixObject{key: "dmcr/_ai_models/direct-upload/objects/session-1/data"},
		fakePrefixObject{key: "dmcr/_ai_models/direct-upload/objects/session-10/data"},
	)
	report := Report{
		StaleDirectUploadPrefixes: []PrefixInventoryEntry{
			{Prefix: "dmcr/_ai_models/direct-upload/objects/session-1"},
		},
	}

	if err := deleteStalePrefixes(context.Background(), store, report); err != nil {
		t.Fatalf("deleteStalePrefixes() error = %v", err)
	}

	if store.hasObject("dmcr/_ai_models/direct-upload/objects/session-1/data") {
		t.Fatal("expected session-1 object to be deleted")
	}
	if !store.hasObject("dmcr/_ai_models/direct-upload/objects/session-10/data") {
		t.Fatal("expected session-10 object to remain")
	}
}

func TestInferDirectUploadPrefixNormalizesPhysicalAndObjectPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "storage object key with root",
			path: "dmcr/_ai_models/direct-upload/objects/session-a/data",
			want: "dmcr/_ai_models/direct-upload/objects/session-a",
		},
		{
			name: "sealed physical path without root",
			path: "/_ai_models/direct-upload/objects/session-b/data",
			want: "dmcr/_ai_models/direct-upload/objects/session-b",
		},
		{
			name: "legacy physical path with root",
			path: "dmcr/_ai_models/direct-upload/objects/session-c/data",
			want: "dmcr/_ai_models/direct-upload/objects/session-c",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, ok := inferDirectUploadPrefix("dmcr", test.path)
			if !ok {
				t.Fatal("inferDirectUploadPrefix() = not found, want found")
			}
			if got != test.want {
				t.Fatalf("inferDirectUploadPrefix() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBuildReportFailsClosedWhenDirectUploadMetadataIsBroken(t *testing.T) {
	t.Parallel()

	store := newFakePrefixStore(
		fakePrefixObject{
			key:          "dmcr/_ai_models/direct-upload/objects/session-a/data",
			lastModified: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		},
		fakePrefixObject{
			key:     "dmcr/docker/registry/v2/blobs/sha256/aa/" + strings.Repeat("a", 64) + "/data.dmcr-sealed",
			payload: []byte("not-json"),
		},
	)

	_, err := buildReportWithClock(
		context.Background(),
		dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			runtime.NewScheme(),
			map[schema.GroupVersionResource]string{
				modelGVR:        "ModelList",
				clusterModelGVR: "ClusterModelList",
			},
		),
		store,
		"dmcr",
		time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC),
		defaultDirectUploadOrphanStaleAge,
	)
	if err == nil {
		t.Fatal("buildReportWithClock() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "decode sealed blob metadata") {
		t.Fatalf("error = %v, want decode sealed blob metadata", err)
	}
}

type fakePrefixObject struct {
	key          string
	lastModified time.Time
	payload      []byte
}

type fakePrefixStore struct {
	objects         map[string]fakePrefixObject
	deletedPrefixes []string
}

func newFakePrefixStore(objects ...fakePrefixObject) *fakePrefixStore {
	result := &fakePrefixStore{objects: make(map[string]fakePrefixObject, len(objects))}
	for _, object := range objects {
		result.objects[strings.Trim(strings.TrimSpace(object.key), "/")] = object
	}
	return result
}

type deletingFakePrefixStore struct {
	*fakePrefixStore
}

func newDeletingFakePrefixStore(objects ...fakePrefixObject) *deletingFakePrefixStore {
	return &deletingFakePrefixStore{fakePrefixStore: newFakePrefixStore(objects...)}
}

func (s *fakePrefixStore) ForEachObject(_ context.Context, prefix string, visit func(string)) error {
	keys := s.matchingKeys(prefix)
	for _, key := range keys {
		visit(key)
	}
	return nil
}

func (s *fakePrefixStore) ForEachObjectInfo(_ context.Context, prefix string, visit func(prefixObjectInfo)) error {
	keys := s.matchingKeys(prefix)
	for _, key := range keys {
		object := s.objects[key]
		visit(prefixObjectInfo{
			Key:          key,
			LastModified: object.lastModified,
		})
	}
	return nil
}

func (s *fakePrefixStore) GetObject(_ context.Context, key string) ([]byte, error) {
	object, found := s.objects[strings.Trim(strings.TrimSpace(key), "/")]
	if !found {
		return nil, fmt.Errorf("object %s not found", key)
	}
	return append([]byte(nil), object.payload...), nil
}

func (s *fakePrefixStore) DeletePrefix(_ context.Context, prefix string) error {
	s.deletedPrefixes = append(s.deletedPrefixes, strings.TrimSpace(prefix))
	return nil
}

func (s *deletingFakePrefixStore) DeletePrefix(_ context.Context, prefix string) error {
	listPrefix := strings.TrimLeft(strings.TrimSpace(prefix), "/")
	s.deletedPrefixes = append(s.deletedPrefixes, strings.TrimSpace(prefix))
	for key := range s.objects {
		if listPrefix == "" || strings.HasPrefix(key, listPrefix) {
			delete(s.objects, key)
		}
	}
	return nil
}

func (s *fakePrefixStore) matchingKeys(prefix string) []string {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	keys := make([]string, 0, len(s.objects))
	for key := range s.objects {
		if cleanPrefix == "" || key == cleanPrefix || strings.HasPrefix(key, cleanPrefix+"/") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (s *deletingFakePrefixStore) hasObject(key string) bool {
	_, found := s.objects[strings.Trim(strings.TrimSpace(key), "/")]
	return found
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
