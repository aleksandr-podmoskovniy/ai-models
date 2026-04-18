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
	"os"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
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
		archive, err := zip.NewReader(zipArchiveSourceReaderAt{
			ctx:        ctx,
			sourcePath: strings.TrimSpace(layer.SourcePath),
			reader:     rangeReader,
			sizeBytes:  layer.Archive.SizeBytes,
		}, layer.Archive.SizeBytes)
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
	if file.Mode()&os.ModeSymlink != 0 {
		return "", false, fmt.Errorf("refusing to publish symbolic link zip entry %q", file.Name)
	}
	normalized, err := archiveRelativePath(file.Name)
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

type zipArchiveSourceReaderAt struct {
	ctx        context.Context
	sourcePath string
	reader     modelpackports.PublishObjectRangeReader
	sizeBytes  int64
}

func (r zipArchiveSourceReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if offset < 0 || offset >= r.sizeBytes {
		return 0, io.EOF
	}

	length := int64(len(p))
	if remaining := r.sizeBytes - offset; length > remaining {
		length = remaining
	}
	output, err := r.reader.OpenReadRange(r.ctx, r.sourcePath, offset, length)
	if err != nil {
		return 0, err
	}
	defer output.Body.Close()

	n, err := io.ReadFull(output.Body, p[:length])
	switch err {
	case nil:
		if int64(n) < int64(len(p)) {
			return n, io.EOF
		}
		return n, nil
	case io.ErrUnexpectedEOF, io.EOF:
		if n == 0 {
			return 0, io.EOF
		}
		return n, io.EOF
	default:
		return n, err
	}
}
