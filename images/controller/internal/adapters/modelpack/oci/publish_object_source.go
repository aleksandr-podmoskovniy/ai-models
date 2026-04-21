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

package oci

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func describeObjectSourcePublishLayer(
	ctx context.Context,
	layer modelpackports.PublishLayer,
	mediaType string,
) (publishLayerDescriptor, error) {
	if err := validateObjectSourceLayer(layer); err != nil {
		return publishLayerDescriptor{}, err
	}
	if layer.Format == modelpackports.LayerFormatRaw {
		return describeRawObjectSourcePublishLayer(ctx, layer, mediaType)
	}
	return describeArchiveObjectSourcePublishLayer(ctx, layer, mediaType)
}

func describeRawObjectSourcePublishLayer(
	ctx context.Context,
	layer modelpackports.PublishLayer,
	mediaType string,
) (publishLayerDescriptor, error) {
	file := layer.ObjectSource.Files[0]
	object, err := layer.ObjectSource.Reader.OpenRead(ctx, file.SourcePath)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	defer object.Body.Close()
	if err := validateOpenedObjectSource(file, object); err != nil {
		return publishLayerDescriptor{}, err
	}

	hasher := sha256.New()
	counter := &countWriter{}
	if _, err := io.Copy(io.MultiWriter(hasher, counter), object.Body); err != nil {
		return publishLayerDescriptor{}, err
	}
	digest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	return publishLayerDescriptor{
		Digest:      digest,
		DiffID:      digest,
		Size:        counter.n,
		MediaType:   mediaType,
		TargetPath:  filepath.ToSlash(strings.TrimSpace(layer.TargetPath)),
		Base:        layer.Base,
		Format:      layer.Format,
		Compression: modelpackports.LayerCompressionNone,
	}, nil
}

func describeArchiveObjectSourcePublishLayer(
	ctx context.Context,
	layer modelpackports.PublishLayer,
	mediaType string,
) (publishLayerDescriptor, error) {
	diffHasher := sha256.New()
	blobHasher := sha256.New()
	counter := &countWriter{}
	compressedSink := io.MultiWriter(blobHasher, counter)

	archiveWriter, err := newArchiveWriter(compressedSink, layer.Compression)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	tarSink := io.MultiWriter(diffHasher, archiveWriter)
	if err := writeLayerArchiveFromObjectSource(ctx, tarSink, layer); err != nil {
		_ = archiveWriter.Close()
		return publishLayerDescriptor{}, err
	}
	if err := archiveWriter.Close(); err != nil {
		return publishLayerDescriptor{}, err
	}

	return publishLayerDescriptor{
		Digest:      "sha256:" + hex.EncodeToString(blobHasher.Sum(nil)),
		DiffID:      "sha256:" + hex.EncodeToString(diffHasher.Sum(nil)),
		Size:        counter.n,
		MediaType:   mediaType,
		TargetPath:  filepath.ToSlash(strings.TrimSpace(layer.TargetPath)),
		Base:        layer.Base,
		Format:      layer.Format,
		Compression: normalizedArchiveCompression(layer.Compression),
	}, nil
}

func openObjectSourceLayerRange(
	ctx context.Context,
	layer modelpackports.PublishLayer,
	offset, length int64,
) (io.ReadCloser, error) {
	if err := validateObjectSourceLayer(layer); err != nil {
		return nil, err
	}
	if layer.Format == modelpackports.LayerFormatRaw {
		return openRawObjectSourceLayerRange(ctx, layer, offset, length)
	}
	if reader, ok := layer.ObjectSource.Reader.(modelpackports.PublishObjectRangeReader); ok &&
		normalizedArchiveCompression(layer.Compression) == modelpackports.LayerCompressionNone {
		return openRangedObjectSourceLayer(ctx, layer, reader, offset, length)
	}

	reader, writer := io.Pipe()
	go func() {
		archiveWriter, openErr := newArchiveWriter(writer, layer.Compression)
		if openErr != nil {
			_ = writer.CloseWithError(openErr)
			return
		}
		writeErr := writeLayerArchiveFromObjectSource(ctx, archiveWriter, layer)
		closeErr := archiveWriter.Close()
		if writeErr != nil {
			_ = writer.CloseWithError(writeErr)
			return
		}
		_ = writer.CloseWithError(closeErr)
	}()

	stream := &archivePipeStream{reader: reader}
	body := io.Reader(stream.reader)
	if offset > 0 {
		body = &offsetReader{reader: body, offset: offset}
	}
	if length >= 0 {
		body = io.LimitReader(body, length)
	}

	return &archiveRangeReader{body: body, stream: stream}, nil
}

func openRawObjectSourceLayerRange(
	ctx context.Context,
	layer modelpackports.PublishLayer,
	offset, length int64,
) (io.ReadCloser, error) {
	file := layer.ObjectSource.Files[0]
	if reader, ok := layer.ObjectSource.Reader.(modelpackports.PublishObjectRangeReader); ok {
		object, err := reader.OpenReadRange(ctx, file.SourcePath, offset, length)
		if err != nil {
			return nil, err
		}
		return object.Body, nil
	}

	object, err := layer.ObjectSource.Reader.OpenRead(ctx, file.SourcePath)
	if err != nil {
		return nil, err
	}
	if err := validateOpenedObjectSource(file, object); err != nil {
		_ = object.Body.Close()
		return nil, err
	}

	body := io.Reader(object.Body)
	if offset > 0 {
		body = &offsetReader{reader: body, offset: offset}
	}
	if length >= 0 {
		body = io.LimitReader(body, length)
	}
	return &wrappedReadCloser{body: body, closer: object.Body}, nil
}

type wrappedReadCloser struct {
	body   io.Reader
	closer io.Closer
}

func (r *wrappedReadCloser) Read(p []byte) (int, error) {
	return r.body.Read(p)
}

func (r *wrappedReadCloser) Close() error {
	return r.closer.Close()
}

func validateObjectSourceLayer(layer modelpackports.PublishLayer) error {
	if layer.ObjectSource == nil {
		return errors.New("object source metadata must not be nil")
	}
	if layer.Archive != nil {
		return errors.New("object source layer must not also declare archive source metadata")
	}
	if layer.ObjectSource.Reader == nil {
		return errors.New("object source reader must not be nil")
	}
	if len(layer.ObjectSource.Files) == 0 {
		return errors.New("object source files must not be empty")
	}
	if layer.Format == modelpackports.LayerFormatRaw {
		return validateRawObjectSourceLayer(layer)
	}
	if layer.Format != modelpackports.LayerFormatTar {
		return fmt.Errorf("object source layer %q must publish as raw or tar", layer.SourcePath)
	}

	seen := make(map[string]struct{}, len(layer.ObjectSource.Files))
	for index, file := range layer.ObjectSource.Files {
		if strings.TrimSpace(file.SourcePath) == "" {
			return fmt.Errorf("object source files[%d] source path must not be empty", index)
		}
		if file.SizeBytes < 0 {
			return fmt.Errorf("object source files[%d] size must not be negative", index)
		}
		normalizedTarget, err := archiveRelativePath(file.TargetPath)
		if err != nil {
			return fmt.Errorf("object source files[%d] target path is invalid: %w", index, err)
		}
		if normalizedTarget == "." {
			return fmt.Errorf("object source files[%d] target path must not be empty", index)
		}
		normalizedTarget = filepath.ToSlash(strings.TrimSpace(normalizedTarget))
		if _, exists := seen[normalizedTarget]; exists {
			return fmt.Errorf("object source duplicates target path %q", normalizedTarget)
		}
		seen[normalizedTarget] = struct{}{}
	}
	return nil
}

func validateRawObjectSourceLayer(layer modelpackports.PublishLayer) error {
	if layer.Compression != "" && layer.Compression != modelpackports.LayerCompressionNone {
		return fmt.Errorf("raw object source layer %q must not declare compression", layer.SourcePath)
	}
	if len(layer.ObjectSource.Files) != 1 {
		return fmt.Errorf("raw object source layer %q must contain exactly one file", layer.SourcePath)
	}

	file := layer.ObjectSource.Files[0]
	if strings.TrimSpace(file.SourcePath) == "" {
		return errors.New("object source files[0] source path must not be empty")
	}
	if file.SizeBytes < 0 {
		return errors.New("object source files[0] size must not be negative")
	}
	normalizedTarget, err := archiveRelativePath(file.TargetPath)
	if err != nil {
		return fmt.Errorf("object source files[0] target path is invalid: %w", err)
	}
	if normalizedTarget == "." {
		return errors.New("object source files[0] target path must not be empty")
	}
	if got, want := filepath.ToSlash(strings.TrimSpace(layer.TargetPath)), filepath.ToSlash(strings.TrimSpace(normalizedTarget)); got != want {
		return fmt.Errorf("raw object source layer target path %q must match file target path %q", layer.TargetPath, normalizedTarget)
	}
	return nil
}

func writeLayerArchiveFromObjectSource(ctx context.Context, writer io.Writer, layer modelpackports.PublishLayer) error {
	tarWriter := tar.NewWriter(writer)
	for _, file := range layer.ObjectSource.Files {
		object, err := layer.ObjectSource.Reader.OpenRead(ctx, file.SourcePath)
		if err != nil {
			return err
		}

		targetPath, err := archiveRelativePath(file.TargetPath)
		if err != nil {
			_ = object.Body.Close()
			return err
		}
		if err := validateOpenedObjectSource(file, object); err != nil {
			_ = object.Body.Close()
			return err
		}
		header := &tar.Header{
			Name: filepath.ToSlash(strings.Trim(strings.TrimSpace(layer.TargetPath), "/") + "/" + strings.Trim(strings.TrimSpace(targetPath), "/")),
			Mode: 0o644,
			Size: file.SizeBytes,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = object.Body.Close()
			return err
		}
		if _, err := io.CopyN(tarWriter, object.Body, file.SizeBytes); err != nil {
			_ = object.Body.Close()
			return err
		}
		if err := object.Body.Close(); err != nil {
			return err
		}
	}
	return tarWriter.Close()
}

func validateOpenedObjectSource(file modelpackports.PublishObjectFile, object modelpackports.OpenReadResult) error {
	if object.Body == nil {
		return fmt.Errorf("object source %q returned nil body", file.SourcePath)
	}
	if object.SizeBytes > 0 && file.SizeBytes > 0 && object.SizeBytes != file.SizeBytes {
		return fmt.Errorf("object source %q size %d does not match expected %d", file.SourcePath, object.SizeBytes, file.SizeBytes)
	}
	if strings.TrimSpace(object.ETag) != "" && strings.TrimSpace(file.ETag) != "" && strings.TrimSpace(object.ETag) != strings.TrimSpace(file.ETag) {
		return fmt.Errorf("object source %q etag %q does not match expected %q", file.SourcePath, object.ETag, file.ETag)
	}
	return nil
}
