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
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
)

func safeExtractTar(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, closeArchive, err := archiveio.NewClosableTarReader(archivePath, file)
	if err != nil {
		return err
	}
	defer func() { _ = closeArchive() }()

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		target, err := archiveio.TargetPath(destination, header.Name)
		if err != nil {
			return err
		}
		if err := archiveio.ExtractTarEntry(reader, header, target); err != nil {
			return err
		}
	}
}

func safeExtractZip(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	for _, file := range archive.File {
		target, err := archiveio.TargetPath(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if archiveio.IsZipSymlink(file) {
			return fmt.Errorf("refusing to extract symbolic link zip entry %q", file.Name)
		}

		reader, err := file.Open()
		if err != nil {
			return err
		}
		if err := archiveio.WriteExtractedFile(target, reader); err != nil {
			reader.Close()
			return err
		}
		if err := reader.Close(); err != nil {
			return err
		}
	}

	return nil
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
