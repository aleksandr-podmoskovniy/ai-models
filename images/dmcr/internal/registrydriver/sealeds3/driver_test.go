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

package sealeds3

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

type fakeStorageDriver struct {
	files    map[string][]byte
	statDirs map[string]bool
}

func newFakeStorageDriver() *fakeStorageDriver {
	return &fakeStorageDriver{
		files:    make(map[string][]byte),
		statDirs: make(map[string]bool),
	}
}

func (d *fakeStorageDriver) Name() string {
	return "fake"
}

func (d *fakeStorageDriver) GetContent(_ context.Context, path string) ([]byte, error) {
	payload, ok := d.files[path]
	if !ok {
		return nil, storagedriver.PathNotFoundError{Path: path, DriverName: d.Name()}
	}
	return append([]byte(nil), payload...), nil
}

func (d *fakeStorageDriver) PutContent(_ context.Context, path string, content []byte) error {
	d.files[path] = append([]byte(nil), content...)
	return nil
}

func (d *fakeStorageDriver) Reader(_ context.Context, path string, offset int64) (io.ReadCloser, error) {
	payload, ok := d.files[path]
	if !ok {
		return nil, storagedriver.PathNotFoundError{Path: path, DriverName: d.Name()}
	}
	if offset > int64(len(payload)) {
		offset = int64(len(payload))
	}
	return io.NopCloser(bytes.NewReader(payload[offset:])), nil
}

func (d *fakeStorageDriver) Writer(context.Context, string, bool) (storagedriver.FileWriter, error) {
	return nil, storagedriver.ErrUnsupportedMethod{DriverName: d.Name()}
}

func (d *fakeStorageDriver) Stat(_ context.Context, path string) (storagedriver.FileInfo, error) {
	payload, ok := d.files[path]
	if ok {
		return storagedriver.FileInfoInternal{FileInfoFields: storagedriver.FileInfoFields{
			Path:    path,
			Size:    int64(len(payload)),
			ModTime: time.Unix(1_700_000_000, 0),
			IsDir:   false,
		}}, nil
	}
	if d.statDirs[path] {
		return storagedriver.FileInfoInternal{FileInfoFields: storagedriver.FileInfoFields{
			Path:    path,
			Size:    0,
			ModTime: time.Unix(1_700_000_000, 0),
			IsDir:   true,
		}}, nil
	}
	return nil, storagedriver.PathNotFoundError{Path: path, DriverName: d.Name()}
}

func (d *fakeStorageDriver) List(_ context.Context, path string) ([]string, error) {
	children := make([]string, 0)
	prefix := strings.TrimSuffix(path, "/") + "/"
	for currentPath := range d.files {
		if strings.HasPrefix(currentPath, prefix) {
			children = append(children, currentPath)
		}
	}
	slices.Sort(children)
	return children, nil
}

func (d *fakeStorageDriver) Move(context.Context, string, string) error {
	return storagedriver.ErrUnsupportedMethod{DriverName: d.Name()}
}

func (d *fakeStorageDriver) Delete(_ context.Context, path string) error {
	delete(d.files, path)
	return nil
}

func (d *fakeStorageDriver) RedirectURL(_ *http.Request, path string) (string, error) {
	return "https://storage.example" + path, nil
}

func (d *fakeStorageDriver) Walk(_ context.Context, path string, walkFn storagedriver.WalkFn, _ ...func(*storagedriver.WalkOptions)) error {
	paths := make([]string, 0, len(d.files))
	for currentPath := range d.files {
		if strings.HasPrefix(currentPath, path) {
			paths = append(paths, currentPath)
		}
	}
	slices.Sort(paths)
	for _, currentPath := range paths {
		if err := walkFn(storagedriver.FileInfoInternal{FileInfoFields: storagedriver.FileInfoFields{
			Path:    currentPath,
			Size:    int64(len(d.files[currentPath])),
			ModTime: time.Unix(1_700_000_000, 0),
			IsDir:   false,
		}}); err != nil {
			return err
		}
	}
	return nil
}

func TestStorageDriverServesResolvedBlob(t *testing.T) {
	t.Parallel()

	const (
		canonicalPath = "/docker/registry/v2/blobs/sha256/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/data"
		physicalPath  = "/_ai_models/direct-upload/objects/session-1/data"
	)

	delegate := newFakeStorageDriver()
	delegate.files[physicalPath] = []byte("payload-bytes")
	metadataPayload, err := sealedblob.Marshal(sealedblob.Metadata{
		Version:      sealedblob.MetadataVersion,
		Digest:       "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PhysicalPath: physicalPath,
		SizeBytes:    int64(len(delegate.files[physicalPath])),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	delegate.files[sealedblob.MetadataPath(canonicalPath)] = metadataPayload

	driver := newStorageDriver(delegate)

	fileInfo, err := driver.Stat(context.Background(), canonicalPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got, want := fileInfo.Size(), int64(len(delegate.files[physicalPath])); got != want {
		t.Fatalf("Stat().Size() = %d, want %d", got, want)
	}

	reader, err := driver.Reader(context.Background(), canonicalPath, 0)
	if err != nil {
		t.Fatalf("Reader() error = %v", err)
	}
	defer reader.Close()

	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if got, want := string(payload), "payload-bytes"; got != want {
		t.Fatalf("payload = %q, want %q", got, want)
	}
}

func TestStorageDriverStatResolvesCanonicalBlobWhenDelegateReturnsDirectory(t *testing.T) {
	t.Parallel()

	const (
		canonicalPath = "/docker/registry/v2/blobs/sha256/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/data"
		physicalPath  = "/_ai_models/direct-upload/objects/session-dir/data"
	)

	delegate := newFakeStorageDriver()
	delegate.files[physicalPath] = []byte("payload-bytes")
	delegate.statDirs[canonicalPath] = true
	metadataPayload, err := sealedblob.Marshal(sealedblob.Metadata{
		Version:      sealedblob.MetadataVersion,
		Digest:       "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PhysicalPath: physicalPath,
		SizeBytes:    int64(len(delegate.files[physicalPath])),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	delegate.files[sealedblob.MetadataPath(canonicalPath)] = metadataPayload

	driver := newStorageDriver(delegate)

	fileInfo, err := driver.Stat(context.Background(), canonicalPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if fileInfo.IsDir() {
		t.Fatal("Stat().IsDir() = true, want false")
	}
	if got, want := fileInfo.Size(), int64(len(delegate.files[physicalPath])); got != want {
		t.Fatalf("Stat().Size() = %d, want %d", got, want)
	}
}

func TestStorageDriverDeleteRemovesPhysicalObjectAndMetadata(t *testing.T) {
	t.Parallel()

	const (
		canonicalPath = "/docker/registry/v2/blobs/sha256/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/data"
		physicalPath  = "/_ai_models/direct-upload/objects/session-2/data"
	)

	delegate := newFakeStorageDriver()
	delegate.files[physicalPath] = []byte("payload")
	metadataPayload, err := sealedblob.Marshal(sealedblob.Metadata{
		Version:      sealedblob.MetadataVersion,
		Digest:       "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PhysicalPath: physicalPath,
		SizeBytes:    7,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	delegate.files[sealedblob.MetadataPath(canonicalPath)] = metadataPayload

	driver := newStorageDriver(delegate)
	if err := driver.Delete(context.Background(), canonicalPath); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := delegate.files[physicalPath]; ok {
		t.Fatal("physical object still exists after Delete()")
	}
	if _, ok := delegate.files[sealedblob.MetadataPath(canonicalPath)]; ok {
		t.Fatal("metadata object still exists after Delete()")
	}
}

func TestStorageDriverWalkProjectsMetadataAsCanonicalBlob(t *testing.T) {
	t.Parallel()

	const canonicalPath = "/docker/registry/v2/blobs/sha256/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/data"

	delegate := newFakeStorageDriver()
	metadataPayload, err := sealedblob.Marshal(sealedblob.Metadata{
		Version:      sealedblob.MetadataVersion,
		Digest:       "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PhysicalPath: "/_ai_models/direct-upload/objects/session-3/data",
		SizeBytes:    42,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	delegate.files[sealedblob.MetadataPath(canonicalPath)] = metadataPayload

	driver := newStorageDriver(delegate)
	visited := make([]string, 0)
	if err := driver.Walk(context.Background(), "/docker/registry/v2/blobs", func(fileInfo storagedriver.FileInfo) error {
		visited = append(visited, fileInfo.Path())
		return nil
	}); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if len(visited) != 1 || visited[0] != canonicalPath {
		t.Fatalf("Walk() visited %#v, want [%q]", visited, canonicalPath)
	}
}

func TestStorageDriverRejectsMismatchedMetadataDigest(t *testing.T) {
	t.Parallel()

	const canonicalPath = "/docker/registry/v2/blobs/sha256/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/data"

	delegate := newFakeStorageDriver()
	metadataPayload, err := sealedblob.Marshal(sealedblob.Metadata{
		Version:      sealedblob.MetadataVersion,
		Digest:       "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		PhysicalPath: "/_ai_models/direct-upload/objects/session-4/data",
		SizeBytes:    42,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	delegate.files[sealedblob.MetadataPath(canonicalPath)] = metadataPayload

	driver := newStorageDriver(delegate)
	_, err = driver.Stat(context.Background(), canonicalPath)
	if err == nil {
		t.Fatal("Stat() error = nil, want metadata digest mismatch error")
	}
}
