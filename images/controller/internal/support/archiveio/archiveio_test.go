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

package archiveio

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

func TestRangeReaderAtReadsRangesAndReportsEOF(t *testing.T) {
	t.Parallel()

	payload := []byte("abcdef")
	reader := NewRangeReaderAt(context.Background(), "model.zip", int64(len(payload)), func(_ context.Context, sourcePath string, offset, length int64) (io.ReadCloser, error) {
		if sourcePath != "model.zip" {
			t.Fatalf("unexpected source path %q", sourcePath)
		}
		end := offset + length
		if offset < 0 || end > int64(len(payload)) {
			t.Fatalf("unexpected range offset=%d length=%d", offset, length)
		}
		return io.NopCloser(bytes.NewReader(payload[offset:end])), nil
	})

	buffer := make([]byte, 3)
	n, err := reader.ReadAt(buffer, 1)
	if err != nil {
		t.Fatalf("read middle range: %v", err)
	}
	if n != 3 || string(buffer) != "bcd" {
		t.Fatalf("unexpected middle range n=%d data=%q", n, buffer)
	}

	buffer = make([]byte, 4)
	n, err = reader.ReadAt(buffer, 4)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("tail range error = %v, want EOF", err)
	}
	if n != 2 || string(buffer[:n]) != "ef" {
		t.Fatalf("unexpected tail range n=%d data=%q", n, buffer[:n])
	}

	n, err = reader.ReadAt(make([]byte, 1), int64(len(payload)))
	if !errors.Is(err, io.EOF) || n != 0 {
		t.Fatalf("past-end read n=%d err=%v, want 0 EOF", n, err)
	}
}

func TestNewClosableTarReaderSelectsGzipTar(t *testing.T) {
	t.Parallel()

	archive := gzipTarArchive(t, "model/config.json", `{"model":"demo"}`)
	reader, closeReader, err := NewClosableTarReader("MODEL.TGZ", bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("new gzip tar reader: %v", err)
	}
	defer func() {
		if err := closeReader(); err != nil {
			t.Fatalf("close gzip tar reader: %v", err)
		}
	}()

	header, err := reader.Next()
	if err != nil {
		t.Fatalf("read tar header: %v", err)
	}
	if header.Name != "model/config.json" {
		t.Fatalf("header name = %q", header.Name)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read tar content: %v", err)
	}
	if string(content) != `{"model":"demo"}` {
		t.Fatalf("content = %q", content)
	}
}

func TestIsZipSymlinkDetectsMode(t *testing.T) {
	t.Parallel()

	if !IsZipSymlink(zipMode{mode: os.ModeSymlink | 0o777}) {
		t.Fatal("zip symlink mode was not detected")
	}
	if IsZipSymlink(zipMode{mode: 0o644}) {
		t.Fatal("regular zip mode detected as symlink")
	}
}

type zipMode struct {
	mode os.FileMode
}

func (z zipMode) Mode() os.FileMode {
	return z.mode
}

func gzipTarArchive(t *testing.T, name, content string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tarWriter.Write([]byte(content)); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buffer.Bytes()
}
