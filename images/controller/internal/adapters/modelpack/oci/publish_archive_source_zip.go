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
	"archive/zip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
)

func writeZipArchiveLayerFromArchiveSource(ctx context.Context, writer io.Writer, layer modelpackports.PublishLayer) error {
	files, closeArchive, err := openZipArchiveSource(ctx, layer)
	if err != nil {
		return err
	}
	defer func() { _ = closeArchive() }()

	tarWriter := tar.NewWriter(writer)
	selected := archiveSelectedFiles(layer.Archive)
	for _, file := range files {
		relativePath, include, err := selectZipArchiveSourceEntry(file, layer)
		if err != nil {
			return err
		}
		if !include {
			continue
		}
		if len(selected) > 0 {
			if _, ok := selected[relativePath]; !ok {
				continue
			}
		}
		stream, err := file.Open()
		if err != nil {
			return err
		}
		err = writeArchiveSourceFileEntry(
			tarWriter,
			stream,
			filepath.ToSlash(strings.Trim(strings.TrimSpace(layer.TargetPath), "/"))+"/"+relativePath,
			&tar.Header{Mode: int64(file.Mode().Perm()), Size: int64(file.UncompressedSize64)},
		)
		closeErr := stream.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return tarWriter.Close()
}

func openZipArchiveSource(ctx context.Context, layer modelpackports.PublishLayer) ([]*zip.File, func() error, error) {
	if layer.Archive != nil && layer.Archive.Reader != nil {
		rangeReader := layer.Archive.Reader.(modelpackports.PublishObjectRangeReader)
		archive, err := zip.NewReader(archiveio.NewRangeReaderAt(
			ctx,
			layer.SourcePath,
			layer.Archive.SizeBytes,
			func(ctx context.Context, sourcePath string, offset, length int64) (io.ReadCloser, error) {
				output, err := rangeReader.OpenReadRange(ctx, sourcePath, offset, length)
				if err != nil {
					return nil, err
				}
				return output.Body, nil
			},
		), layer.Archive.SizeBytes)
		if err != nil {
			return nil, nil, err
		}
		return archive.File, func() error { return nil }, nil
	}

	archive, err := zip.OpenReader(layer.SourcePath)
	if err != nil {
		return nil, nil, err
	}
	return archive.File, archive.Close, nil
}

func selectZipArchiveSourceEntry(file *zip.File, layer modelpackports.PublishLayer) (string, bool, error) {
	if file.FileInfo().IsDir() {
		return "", false, nil
	}
	if archiveio.IsZipSymlink(file) {
		return "", false, fmt.Errorf("refusing to publish symbolic link zip entry %q", file.Name)
	}
	normalized, err := archiveio.RelativePath(file.Name)
	if err != nil {
		return "", false, err
	}
	prefix := strings.Trim(strings.TrimSpace(layer.Archive.StripPathPrefix), "/")
	if prefix != "" {
		prefix += "/"
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)
		}
	}
	normalized = strings.Trim(strings.TrimSpace(filepath.ToSlash(normalized)), "/")
	if normalized == "" || normalized == "." {
		return "", false, nil
	}
	return normalized, true, nil
}
