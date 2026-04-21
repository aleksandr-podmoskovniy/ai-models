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
	"context"
	"errors"
	"io"
	"net/http"
	"slices"

	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/distribution/distribution/v3/registry/storage/driver/factory"
	s3driver "github.com/distribution/distribution/v3/registry/storage/driver/s3-aws"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

const (
	driverName = "sealeds3"
)

func init() {
	factory.Register(driverName, &storageDriverFactory{})
}

type storageDriverFactory struct{}

func (factory *storageDriverFactory) Create(ctx context.Context, parameters map[string]interface{}) (storagedriver.StorageDriver, error) {
	delegate, err := s3driver.FromParameters(ctx, parameters)
	if err != nil {
		return nil, err
	}
	return newStorageDriver(delegate), nil
}

type storageDriver struct {
	delegate storagedriver.StorageDriver
}

func newStorageDriver(delegate storagedriver.StorageDriver) storagedriver.StorageDriver {
	return &storageDriver{delegate: delegate}
}

func (d *storageDriver) Name() string {
	return driverName
}

func (d *storageDriver) GetContent(ctx context.Context, path string) ([]byte, error) {
	payload, err := d.delegate.GetContent(ctx, path)
	if err == nil || !isBlobResolutionCandidate(path) || !isPathNotFound(err) {
		return payload, err
	}

	reader, _, err := d.resolvedReader(ctx, path, 0)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (d *storageDriver) PutContent(ctx context.Context, path string, content []byte) error {
	return d.delegate.PutContent(ctx, path, content)
}

func (d *storageDriver) Reader(ctx context.Context, path string, offset int64) (io.ReadCloser, error) {
	reader, err := d.delegate.Reader(ctx, path, offset)
	if err == nil || !isBlobResolutionCandidate(path) || !isPathNotFound(err) {
		return reader, err
	}

	reader, _, err = d.resolvedReader(ctx, path, offset)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func (d *storageDriver) Writer(ctx context.Context, path string, append bool) (storagedriver.FileWriter, error) {
	return d.delegate.Writer(ctx, path, append)
}

func (d *storageDriver) Stat(ctx context.Context, path string) (storagedriver.FileInfo, error) {
	fileInfo, err := d.delegate.Stat(ctx, path)
	if err == nil || !isBlobResolutionCandidate(path) || !isPathNotFound(err) {
		return fileInfo, err
	}

	metadata, _, err := d.loadMetadata(ctx, path)
	if err != nil {
		return nil, err
	}
	physicalInfo, err := d.delegate.Stat(ctx, metadata.PhysicalPath)
	if err != nil {
		return nil, err
	}
	return storagedriver.FileInfoInternal{FileInfoFields: storagedriver.FileInfoFields{
		Path:    path,
		Size:    metadata.SizeBytes,
		ModTime: physicalInfo.ModTime(),
		IsDir:   false,
	}}, nil
}

func (d *storageDriver) List(ctx context.Context, path string) ([]string, error) {
	paths, err := d.delegate.List(ctx, path)
	if err != nil {
		return nil, err
	}

	normalized := make([]string, 0, len(paths))
	for _, currentPath := range paths {
		if canonicalPath, ok := sealedblob.CanonicalPathFromMetadataPath(currentPath); ok {
			normalized = append(normalized, canonicalPath)
			continue
		}
		normalized = append(normalized, currentPath)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized), nil
}

func (d *storageDriver) Move(ctx context.Context, sourcePath, destPath string) error {
	return d.delegate.Move(ctx, sourcePath, destPath)
}

func (d *storageDriver) Delete(ctx context.Context, path string) error {
	if !isBlobResolutionCandidate(path) {
		return d.delegate.Delete(ctx, path)
	}

	metadata, metadataPath, err := d.loadMetadata(ctx, path)
	if err != nil {
		if isPathNotFound(err) {
			return d.delegate.Delete(ctx, path)
		}
		return err
	}

	var deleteErrs []error
	if err := d.delegate.Delete(ctx, metadata.PhysicalPath); err != nil && !isPathNotFound(err) {
		deleteErrs = append(deleteErrs, err)
	}
	if err := d.delegate.Delete(ctx, metadataPath); err != nil && !isPathNotFound(err) {
		deleteErrs = append(deleteErrs, err)
	}
	if err := d.delegate.Delete(ctx, path); err != nil && !isPathNotFound(err) {
		deleteErrs = append(deleteErrs, err)
	}
	return errors.Join(deleteErrs...)
}

func (d *storageDriver) RedirectURL(r *http.Request, path string) (string, error) {
	if !isBlobResolutionCandidate(path) {
		return d.delegate.RedirectURL(r, path)
	}

	metadata, _, err := d.loadMetadata(r.Context(), path)
	if err == nil {
		return d.delegate.RedirectURL(r, metadata.PhysicalPath)
	}
	if !isPathNotFound(err) {
		return "", err
	}
	return d.delegate.RedirectURL(r, path)
}

func (d *storageDriver) Walk(ctx context.Context, path string, walkFn storagedriver.WalkFn, options ...func(*storagedriver.WalkOptions)) error {
	return d.delegate.Walk(ctx, path, func(fileInfo storagedriver.FileInfo) error {
		if fileInfo.IsDir() || !sealedblob.IsMetadataPath(fileInfo.Path()) {
			return walkFn(fileInfo)
		}

		metadata, err := d.readMetadataAtPath(ctx, fileInfo.Path())
		if err != nil {
			return err
		}
		canonicalPath, ok := sealedblob.CanonicalPathFromMetadataPath(fileInfo.Path())
		if !ok {
			return walkFn(fileInfo)
		}
		return walkFn(storagedriver.FileInfoInternal{FileInfoFields: storagedriver.FileInfoFields{
			Path:    canonicalPath,
			Size:    metadata.SizeBytes,
			ModTime: fileInfo.ModTime(),
			IsDir:   false,
		}})
	}, options...)
}

func (d *storageDriver) resolvedReader(ctx context.Context, path string, offset int64) (io.ReadCloser, sealedblob.Metadata, error) {
	metadata, _, err := d.loadMetadata(ctx, path)
	if err != nil {
		return nil, sealedblob.Metadata{}, err
	}
	reader, err := d.delegate.Reader(ctx, metadata.PhysicalPath, offset)
	if err != nil {
		return nil, sealedblob.Metadata{}, err
	}
	return reader, metadata, nil
}

func (d *storageDriver) loadMetadata(ctx context.Context, blobDataPath string) (sealedblob.Metadata, string, error) {
	metadataPath := sealedblob.MetadataPath(blobDataPath)
	metadata, err := d.readMetadataAtPath(ctx, metadataPath)
	if err != nil {
		return sealedblob.Metadata{}, "", err
	}
	expectedDigest, ok := sealedblob.DigestFromCanonicalBlobDataPath(blobDataPath)
	if !ok {
		return sealedblob.Metadata{}, "", storagedriver.InvalidPathError{Path: blobDataPath, DriverName: d.Name()}
	}
	if metadata.Digest != expectedDigest {
		return sealedblob.Metadata{}, "", storagedriver.InvalidPathError{Path: metadataPath, DriverName: d.Name()}
	}
	return metadata, metadataPath, nil
}

func (d *storageDriver) readMetadataAtPath(ctx context.Context, metadataPath string) (sealedblob.Metadata, error) {
	payload, err := d.delegate.GetContent(ctx, metadataPath)
	if err != nil {
		return sealedblob.Metadata{}, err
	}
	return sealedblob.Unmarshal(payload)
}

func isBlobResolutionCandidate(path string) bool {
	return sealedblob.LooksLikeCanonicalBlobDataPath(path)
}

func isPathNotFound(err error) bool {
	var pathNotFound storagedriver.PathNotFoundError
	return errors.As(err, &pathNotFound)
}
