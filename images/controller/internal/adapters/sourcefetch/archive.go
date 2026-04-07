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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func PrepareModelInput(sourcePath, destination string) (string, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return "", errors.New("input source path must not be empty")
	}
	if strings.TrimSpace(destination) == "" {
		return "", errors.New("input destination must not be empty")
	}

	switch {
	case isTarArchive(sourcePath), isZipArchive(sourcePath):
		return UnpackArchive(sourcePath, destination)
	default:
		return materializeSingleFile(sourcePath, destination)
	}
}

func UnpackArchive(archivePath, destination string) (string, error) {
	if strings.TrimSpace(archivePath) == "" {
		return "", errors.New("archive path must not be empty")
	}
	if strings.TrimSpace(destination) == "" {
		return "", errors.New("archive destination must not be empty")
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", err
	}

	switch {
	case isTarArchive(archivePath):
		if err := safeExtractTar(archivePath, destination); err != nil {
			return "", err
		}
	case isZipArchive(archivePath):
		if err := safeExtractZip(archivePath, destination); err != nil {
			return "", err
		}
	default:
		return "", errors.New("archive must be .tar, .tar.gz, .tgz, or .zip")
	}

	return normalizeExtractedRoot(destination)
}

func safeExtractTar(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, err := newTarReader(archivePath, file)
	if err != nil {
		return err
	}

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		target, err := archiveTargetPath(destination, header.Name)
		if err != nil {
			return err
		}
		if err := extractTarEntry(reader, header, target); err != nil {
			return err
		}
	}
}

func extractTarEntry(reader *tar.Reader, header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		return writeExtractedFile(target, reader)
	case tar.TypeSymlink:
		return fmt.Errorf("refusing to extract symbolic link tar entry %q", header.Name)
	case tar.TypeLink:
		return fmt.Errorf("refusing to extract hard link tar entry %q", header.Name)
	default:
		return fmt.Errorf("refusing to extract unsupported tar entry %q", header.Name)
	}
}

func writeExtractedFile(target string, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	stream, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(stream, reader); err != nil {
		stream.Close()
		return err
	}
	return stream.Close()
}

func safeExtractZip(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	for _, file := range archive.File {
		target, err := archiveTargetPath(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if isZipSymlink(file) {
			return fmt.Errorf("refusing to extract symbolic link zip entry %q", file.Name)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		stream, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err != nil {
			reader.Close()
			return err
		}
		if _, err := io.Copy(stream, reader); err != nil {
			reader.Close()
			stream.Close()
			return err
		}
		if err := reader.Close(); err != nil {
			stream.Close()
			return err
		}
		if err := stream.Close(); err != nil {
			return err
		}
	}

	return nil
}

func archiveTargetPath(destination, name string) (string, error) {
	relative, err := archiveRelativePath(name)
	if err != nil {
		return "", err
	}
	if relative == "." {
		return destination, nil
	}
	return filepath.Join(destination, relative), nil
}

func archiveRelativePath(name string) (string, error) {
	rawName := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if rawName == "" {
		return "", errors.New("archive entry name must not be empty")
	}

	parts := strings.Split(rawName, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("refusing to extract archive entry outside of destination: %q", name)
		default:
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return ".", nil
	}
	return filepath.Join(result...), nil
}

func normalizeExtractedRoot(destination string) (string, error) {
	entries, err := os.ReadDir(destination)
	if err != nil {
		return "", err
	}

	meaningful := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".DS_Store" || entry.Name() == "__MACOSX" {
			continue
		}
		meaningful = append(meaningful, entry)
	}
	if len(meaningful) == 1 && meaningful[0].IsDir() {
		return filepath.Join(destination, meaningful[0].Name()), nil
	}
	return destination, nil
}

func materializeSingleFile(sourcePath, destination string) (string, error) {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", err
	}

	name, err := normalizedInputFileName(sourcePath)
	if err != nil {
		return "", err
	}

	target := filepath.Join(destination, name)
	if err := copyFile(sourcePath, target); err != nil {
		return "", err
	}

	return destination, nil
}

func normalizedInputFileName(sourcePath string) (string, error) {
	base := strings.TrimSpace(filepath.Base(sourcePath))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "", errors.New("input source file name must not be empty")
	}

	if strings.HasSuffix(strings.ToLower(base), ".gguf") {
		return base, nil
	}

	looksLikeGGUF, err := hasGGUFMagic(sourcePath)
	if err != nil {
		return "", err
	}
	if looksLikeGGUF {
		return base + ".gguf", nil
	}

	return base, nil
}

func hasGGUFMagic(path string) (bool, error) {
	stream, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer stream.Close()

	header := make([]byte, 4)
	n, err := io.ReadFull(stream, header)
	switch {
	case err == nil:
		return n == 4 && string(header) == "GGUF", nil
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return false, nil
	default:
		return false, err
	}
}

func copyFile(sourcePath, target string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	stream, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(stream, source); err != nil {
		stream.Close()
		return err
	}
	return stream.Close()
}
