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

package sourcefetch

import (
	"archive/tar"
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeExtractTarRejectsSymbolicLinks(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "model.tar")

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	writer := tar.NewWriter(file)
	link := &tar.Header{Name: "checkpoint-link", Typeflag: tar.TypeSymlink, Linkname: "../escape"}
	if err := writer.WriteHeader(link); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := UnpackArchive(archivePath, filepath.Join(tempDir, "out")); err == nil {
		t.Fatal("expected symbolic link extraction to fail")
	}
}

func TestSafeExtractZipRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "model.zip")

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	writer := zip.NewWriter(file)
	stream, err := writer.Create("../escape.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := stream.Write([]byte("boom")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := UnpackArchive(archivePath, filepath.Join(tempDir, "out")); err == nil {
		t.Fatal("expected path traversal extraction to fail")
	}
}

func TestUnpackArchiveExtractsRegularCheckpointArchive(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "model.tar")

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	writer := tar.NewWriter(file)
	content := []byte(`{"model_type":"llama"}`)
	header := &tar.Header{Name: "checkpoint/config.json", Mode: 0o644, Size: int64(len(content))}
	if err := writer.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	root, err := UnpackArchive(archivePath, filepath.Join(tempDir, "out"))
	if err != nil {
		t.Fatalf("UnpackArchive() error = %v", err)
	}
	if got, want := root, filepath.Join(tempDir, "out", "checkpoint"); got != want {
		t.Fatalf("unexpected extracted root %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "config.json")); err != nil {
		t.Fatalf("Stat(config.json) error = %v", err)
	}
}

func TestArchiveRelativePathRejectsTraversal(t *testing.T) {
	t.Parallel()

	if _, err := archiveRelativePath("../escape.txt"); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestNewTarReaderSupportsGzip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "model.tgz")
	if err := createGzipTar(archivePath, "checkpoint/config.json", []byte(`{}`)); err != nil {
		t.Fatalf("createGzipTar() error = %v", err)
	}

	root, err := UnpackArchive(archivePath, filepath.Join(tempDir, "out"))
	if err != nil {
		t.Fatalf("UnpackArchive() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "config.json")); err != nil {
		t.Fatalf("Stat(config.json) error = %v", err)
	}
}

func TestPrepareModelInputLinksGGUFWhenPossibleAndNormalizesExtension(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "model-input")
	if err := os.WriteFile(sourcePath, []byte("GGUFpayload"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	root, err := PrepareModelInput(sourcePath, filepath.Join(tempDir, "out"))
	if err != nil {
		t.Fatalf("PrepareModelInput() error = %v", err)
	}
	if got, want := root, filepath.Join(tempDir, "out"); got != want {
		t.Fatalf("unexpected prepared root %q", got)
	}
	target := filepath.Join(root, "model-input.gguf")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("Stat(model-input.gguf) error = %v", err)
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("Stat(sourcePath) error = %v", err)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat(target) error = %v", err)
	}
	if !os.SameFile(sourceInfo, targetInfo) {
		t.Fatalf("expected model input to be hard-linked when source and destination share the same filesystem")
	}
}
