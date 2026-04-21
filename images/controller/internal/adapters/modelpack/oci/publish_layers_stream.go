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
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func openPublishLayerRange(ctx context.Context, layer modelpackports.PublishLayer, offset, length int64) (io.ReadCloser, error) {
	if layer.ObjectSource != nil {
		return openObjectSourceLayerRange(ctx, layer, offset, length)
	}
	if layer.Format == modelpackports.LayerFormatRaw {
		return openRawLayerRange(layer.SourcePath, offset, length)
	}
	if layer.Archive != nil {
		return openArchiveSourceLayerRange(ctx, layer, offset, length)
	}
	return openArchiveLayerRange(layer, offset, length)
}

func openRawLayerRange(path string, offset, length int64) (io.ReadCloser, error) {
	stream, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := stream.Seek(offset, io.SeekStart); err != nil {
			stream.Close()
			return nil, err
		}
	}
	if length < 0 {
		return stream, nil
	}
	return &limitedFileReader{file: stream, body: io.LimitReader(stream, length)}, nil
}

type limitedFileReader struct {
	file *os.File
	body io.Reader
}

func (r *limitedFileReader) Read(p []byte) (int, error) {
	return r.body.Read(p)
}

func (r *limitedFileReader) Close() error {
	return r.file.Close()
}

func openArchiveLayerRange(layer modelpackports.PublishLayer, offset, length int64) (io.ReadCloser, error) {
	info, err := os.Stat(layer.SourcePath)
	if err != nil {
		return nil, err
	}

	reader, writer := io.Pipe()
	go func() {
		archiveWriter, openErr := newArchiveWriter(writer, layer.Compression)
		if openErr != nil {
			_ = writer.CloseWithError(openErr)
			return
		}
		writeErr := writeLayerArchive(archiveWriter, layer.SourcePath, layer.TargetPath, info)
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

func newArchiveWriter(writer io.Writer, compression modelpackports.LayerCompression) (closeWriter, error) {
	switch normalizedArchiveCompression(compression) {
	case modelpackports.LayerCompressionNone:
		return nopCloseWriter{Writer: writer}, nil
	case modelpackports.LayerCompressionGzip:
		return gzip.NewWriter(writer), nil
	case modelpackports.LayerCompressionGzipFastest:
		return gzip.NewWriterLevel(writer, gzip.BestSpeed)
	case modelpackports.LayerCompressionZstd:
		return zstd.NewWriter(writer)
	default:
		return nil, fmt.Errorf("unsupported archive compression %q", compression)
	}
}

func normalizedArchiveCompression(compression modelpackports.LayerCompression) modelpackports.LayerCompression {
	if compression == "" {
		return modelpackports.LayerCompressionNone
	}
	return compression
}

type nopCloseWriter struct {
	io.Writer
}

func (w nopCloseWriter) Close() error {
	return nil
}
