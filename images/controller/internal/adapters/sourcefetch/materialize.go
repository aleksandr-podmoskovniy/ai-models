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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func materializeSingleFile(sourcePath, destination string) (string, error) {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", err
	}

	name, err := normalizedInputFileName(sourcePath)
	if err != nil {
		return "", err
	}

	target := filepath.Join(destination, name)
	if err := linkOrCopyFile(sourcePath, target); err != nil {
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

func linkOrCopyFile(sourcePath, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.Link(sourcePath, target); err == nil {
		return nil
	}
	return copyFile(sourcePath, target)
}
