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

func validateObjectSourceLayer(layer modelpackports.PublishLayer) error {
	if layer.ObjectSource == nil {
		return errors.New("object source metadata must not be nil")
	}
	if layer.Archive != nil {
		return errors.New("object source layer must not also declare archive source metadata")
	}
	if layer.Format != modelpackports.LayerFormatTar {
		return fmt.Errorf("object source layer %q must publish as tar", layer.SourcePath)
	}
	if layer.ObjectSource.Reader == nil {
		return errors.New("object source reader must not be nil")
	}
	if len(layer.ObjectSource.Files) == 0 {
		return errors.New("object source files must not be empty")
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
