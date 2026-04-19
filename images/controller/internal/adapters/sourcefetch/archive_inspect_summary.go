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
	"path/filepath"
	"strings"
)

func summarizeTarSafetensorsArchiveFromReader(path string, stream io.Reader, rootPrefix string, selectedFiles []string) ([]byte, int64, error) {
	reader, closeArchive, err := newClosableTarReader(path, stream)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = closeArchive() }()

	selected := make(map[string]struct{}, len(selectedFiles))
	for _, file := range selectedFiles {
		selected[strings.TrimSpace(filepath.ToSlash(file))] = struct{}{}
	}

	var (
		configPayload []byte
		weightBytes   int64
	)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, 0, err
		}

		relative, keep, err := classifyArchiveEntry(header)
		if err != nil {
			return nil, 0, err
		}
		if !keep {
			continue
		}
		normalized, ok := normalizedArchiveFilePath(relative, rootPrefix)
		if !ok {
			continue
		}
		if _, ok := selected[normalized]; !ok {
			continue
		}
		switch {
		case normalized == "config.json":
			configPayload, err = io.ReadAll(reader)
			if err != nil {
				return nil, 0, err
			}
		case strings.HasSuffix(strings.ToLower(normalized), ".safetensors"):
			weightBytes += header.Size
		}
	}

	if len(configPayload) == 0 {
		return nil, 0, errors.New("tar safetensors summary requires config.json in selected files")
	}
	if weightBytes <= 0 {
		return nil, 0, errors.New("tar safetensors summary requires positive safetensors weight bytes")
	}
	return configPayload, weightBytes, nil
}

func summarizeGGUFArchive(files []tarArchiveFile, rootPrefix string, selectedFiles []string) (string, int64, error) {
	selected := make(map[string]struct{}, len(selectedFiles))
	for _, file := range selectedFiles {
		selected[strings.TrimSpace(filepath.ToSlash(file))] = struct{}{}
	}

	for _, file := range files {
		normalized, ok := normalizedArchiveFilePath(file.RelativePath, rootPrefix)
		if !ok {
			continue
		}
		if _, ok := selected[normalized]; !ok {
			continue
		}
		if strings.HasSuffix(strings.ToLower(normalized), ".gguf") {
			if file.SizeBytes <= 0 {
				return "", 0, errors.New("gguf archive summary requires positive gguf size")
			}
			return normalized, file.SizeBytes, nil
		}
	}

	return "", 0, errors.New("gguf archive summary requires selected .gguf file")
}
