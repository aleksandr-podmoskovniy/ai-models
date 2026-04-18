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
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/klauspost/compress/zstd"
)

type fakeArchiveLayerReader struct {
	payload []byte
}

func (f fakeArchiveLayerReader) OpenRead(context.Context, string) (modelpackports.OpenReadResult, error) {
	return modelpackports.OpenReadResult{
		Body:      io.NopCloser(bytes.NewReader(f.payload)),
		SizeBytes: int64(len(f.payload)),
		ETag:      `"archive-reader-etag"`,
	}, nil
}

func (f fakeArchiveLayerReader) OpenReadRange(_ context.Context, _ string, offset, length int64) (modelpackports.OpenReadResult, error) {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(f.payload)) {
		offset = int64(len(f.payload))
	}
	end := int64(len(f.payload))
	if length >= 0 && offset+length < end {
		end = offset + length
	}
	return modelpackports.OpenReadResult{
		Body:      io.NopCloser(bytes.NewReader(f.payload[offset:end])),
		SizeBytes: end - offset,
		ETag:      `"archive-reader-etag"`,
	}, nil
}

func writeTestGzipTar(path string, files map[string]string) error {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

func writeTestZip(path string, files map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, content := range files {
		stream, err := writer.Create(name)
		if err != nil {
			return err
		}
		if _, err := stream.Write([]byte(content)); err != nil {
			return err
		}
	}
	return writer.Close()
}

func writeTestZstdTar(path string, files map[string]string) error {
	var buffer bytes.Buffer
	encoder, err := zstd.NewWriter(&buffer)
	if err != nil {
		return err
	}
	writer := tar.NewWriter(encoder)
	for name, content := range files {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}
