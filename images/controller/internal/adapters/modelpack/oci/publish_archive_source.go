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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
)

func describeArchiveSourcePublishLayer(
	ctx context.Context,
	layer modelpackports.PublishLayer,
	mediaType string,
) (publishLayerDescriptor, error) {
	if err := validateArchiveSourceLayer(layer); err != nil {
		return publishLayerDescriptor{}, err
	}

	return describeGeneratedArchiveLayer(layer, mediaType, func(writer io.Writer) error {
		return writeLayerArchiveFromArchiveSource(ctx, writer, layer)
	})
}

func openArchiveSourceLayerRange(ctx context.Context, layer modelpackports.PublishLayer, offset, length int64) (io.ReadCloser, error) {
	return openGeneratedArchiveLayerRange(layer, offset, length, func(writer io.Writer) error {
		return writeLayerArchiveFromArchiveSource(ctx, writer, layer)
	})
}

func validateArchiveSourceLayer(layer modelpackports.PublishLayer) error {
	if layer.Archive == nil {
		return errors.New("archive source metadata must not be nil")
	}
	if layer.Format != modelpackports.LayerFormatTar {
		return fmt.Errorf("archive source layer %q must publish as tar", layer.SourcePath)
	}
	lowerPath := strings.ToLower(strings.TrimSpace(layer.SourcePath))
	if !archiveio.IsTarArchive(lowerPath) && !archiveio.IsZipArchive(lowerPath) {
		return fmt.Errorf("archive source layer %q must point to .tar, .tar.gz, .tgz, .tar.zst, .tar.zstd, .tzst or .zip", layer.SourcePath)
	}
	if layer.Archive.Reader != nil && strings.HasSuffix(lowerPath, ".zip") {
		if _, ok := layer.Archive.Reader.(modelpackports.PublishObjectRangeReader); !ok {
			return fmt.Errorf("archive source layer %q requires ranged reader for zip source", layer.SourcePath)
		}
		if layer.Archive.SizeBytes <= 0 {
			return fmt.Errorf("archive source layer %q requires positive archive size for zip source", layer.SourcePath)
		}
	}
	return nil
}

func writeLayerArchiveFromArchiveSource(ctx context.Context, writer io.Writer, layer modelpackports.PublishLayer) error {
	lowerPath := strings.ToLower(strings.TrimSpace(layer.SourcePath))
	if strings.HasSuffix(lowerPath, ".zip") {
		return writeZipArchiveLayerFromArchiveSource(ctx, writer, layer)
	}

	stream, reader, closeArchive, err := openTarArchiveSource(ctx, layer)
	if err != nil {
		return err
	}
	defer func() {
		_ = closeArchive()
		_ = stream.Close()
	}()

	tarWriter := tar.NewWriter(writer)
	selected := archiveSelectedFiles(layer.Archive)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return tarWriter.Close()
		}
		if err != nil {
			return err
		}

		relativePath, include, err := selectArchiveSourceEntry(header, layer)
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
		if err := writeArchiveSourceFileEntry(tarWriter, reader, filepath.ToSlash(strings.Trim(strings.TrimSpace(layer.TargetPath), "/"))+"/"+relativePath, header); err != nil {
			return err
		}
	}
}

func selectArchiveSourceEntry(header *tar.Header, layer modelpackports.PublishLayer) (string, bool, error) {
	relative, err := archiveio.RelativePath(header.Name)
	if err != nil {
		return "", false, err
	}
	switch header.Typeflag {
	case tar.TypeDir, tar.TypeXHeader, tar.TypeXGlobalHeader:
		return "", false, nil
	case tar.TypeReg, tar.TypeRegA:
	default:
		if header.Typeflag == tar.TypeSymlink {
			return "", false, fmt.Errorf("refusing to publish symbolic link tar entry %q", header.Name)
		}
		if header.Typeflag == tar.TypeLink {
			return "", false, fmt.Errorf("refusing to publish hard link tar entry %q", header.Name)
		}
		return "", false, fmt.Errorf("refusing to publish unsupported tar entry %q", header.Name)
	}

	normalized := filepath.ToSlash(strings.TrimSpace(relative))
	prefix := strings.Trim(strings.TrimSpace(layer.Archive.StripPathPrefix), "/")
	if prefix != "" {
		prefix += "/"
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)
		}
	}
	normalized = strings.Trim(strings.TrimSpace(normalized), "/")
	if normalized == "" || normalized == "." {
		return "", false, nil
	}
	return normalized, true, nil
}

func writeArchiveSourceFileEntry(writer *tar.Writer, reader io.Reader, targetPath string, header *tar.Header) error {
	fileHeader := &tar.Header{
		Name:     filepath.ToSlash(strings.Trim(strings.TrimSpace(targetPath), "/")),
		Typeflag: tar.TypeReg,
		Mode:     header.Mode,
		Size:     header.Size,
	}
	if err := writer.WriteHeader(fileHeader); err != nil {
		return err
	}
	_, err := io.Copy(writer, reader)
	return err
}

func archiveSelectedFiles(archive *modelpackports.PublishArchiveSource) map[string]struct{} {
	if archive == nil || len(archive.SelectedFiles) == 0 {
		return nil
	}
	selected := make(map[string]struct{}, len(archive.SelectedFiles))
	for _, file := range archive.SelectedFiles {
		selected[strings.TrimSpace(filepath.ToSlash(file))] = struct{}{}
	}
	return selected
}

func openTarArchiveSource(ctx context.Context, layer modelpackports.PublishLayer) (io.ReadCloser, *tar.Reader, func() error, error) {
	if layer.Archive != nil && layer.Archive.Reader != nil {
		object, err := layer.Archive.Reader.OpenRead(ctx, strings.TrimSpace(layer.SourcePath))
		if err != nil {
			return nil, nil, nil, err
		}
		reader, closeArchive, err := archiveio.NewClosableTarReader(layer.SourcePath, object.Body)
		if err != nil {
			_ = object.Body.Close()
			return nil, nil, nil, err
		}
		return object.Body, reader, closeArchive, nil
	}
	stream, err := os.Open(layer.SourcePath)
	if err != nil {
		return nil, nil, nil, err
	}
	reader, closeArchive, err := archiveio.NewClosableTarReader(layer.SourcePath, stream)
	if err != nil {
		stream.Close()
		return nil, nil, nil, err
	}
	return stream, reader, closeArchive, nil
}
