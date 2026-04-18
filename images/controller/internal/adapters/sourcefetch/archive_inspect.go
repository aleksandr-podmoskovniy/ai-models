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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
)

type openArchiveReaderFunc func() (io.ReadCloser, error)

type ArchiveInspection struct {
	RootPrefix    string
	InputFormat   modelsv1alpha1.ModelInputFormat
	SelectedFiles []string
	ConfigPayload []byte
	WeightBytes   int64
	ModelFile     string
	ModelFileSize int64
}

type tarArchiveFile struct {
	RelativePath string
	SizeBytes    int64
}

func InspectModelArchive(path string, requested modelsv1alpha1.ModelInputFormat) (ArchiveInspection, error) {
	if strings.TrimSpace(path) == "" {
		return ArchiveInspection{}, errors.New("archive path must not be empty")
	}
	switch {
	case isTarArchive(path):
		return inspectTarModelArchive(path, requested)
	case isZipArchive(path):
		return inspectZipModelArchive(path, requested)
	default:
		return ArchiveInspection{}, errors.New("streaming archive inspection only supports tar/tar.gz/tgz/tar.zst/tar.zstd/tzst/zip")
	}
}

func InspectTarModelArchiveReader(path string, openReader openArchiveReaderFunc, requested modelsv1alpha1.ModelInputFormat) (ArchiveInspection, error) {
	if !isTarArchive(path) {
		return ArchiveInspection{}, errors.New("streaming tar archive inspection only supports tar/tar.gz/tgz/tar.zst/tar.zstd/tzst")
	}
	return inspectTarModelArchiveReader(path, openReader, requested)
}

func inspectTarModelArchive(path string, requested modelsv1alpha1.ModelInputFormat) (ArchiveInspection, error) {
	return inspectTarModelArchiveReader(path, func() (io.ReadCloser, error) {
		return os.Open(path)
	}, requested)
}

func inspectTarModelArchiveReader(path string, openReader openArchiveReaderFunc, requested modelsv1alpha1.ModelInputFormat) (ArchiveInspection, error) {
	stream, err := openReader()
	if err != nil {
		return ArchiveInspection{}, err
	}
	files, rootPrefix, err := listTarArchiveFilesFromReader(path, stream)
	_ = stream.Close()
	if err != nil {
		return ArchiveInspection{}, err
	}

	normalizedFiles := make([]string, 0, len(files))
	for _, file := range files {
		normalized, ok := normalizedArchiveFilePath(file.RelativePath, rootPrefix)
		if !ok {
			continue
		}
		normalizedFiles = append(normalizedFiles, normalized)
	}

	inputFormat, err := resolveRemoteFormat(normalizedFiles, requested)
	if err != nil {
		return ArchiveInspection{}, err
	}
	selectedFiles, err := modelformat.SelectRemoteFiles(inputFormat, normalizedFiles)
	if err != nil {
		return ArchiveInspection{}, err
	}

	inspection := ArchiveInspection{
		RootPrefix:    rootPrefix,
		InputFormat:   inputFormat,
		SelectedFiles: selectedFiles,
	}
	switch inputFormat {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		stream, err := openReader()
		if err != nil {
			return ArchiveInspection{}, err
		}
		configPayload, weightBytes, err := summarizeTarSafetensorsArchiveFromReader(path, stream, rootPrefix, selectedFiles)
		_ = stream.Close()
		if err != nil {
			return ArchiveInspection{}, err
		}
		inspection.ConfigPayload = configPayload
		inspection.WeightBytes = weightBytes
		return inspection, nil
	case modelsv1alpha1.ModelInputFormatGGUF:
		modelFile, modelFileSize, err := summarizeGGUFArchive(files, rootPrefix, selectedFiles)
		if err != nil {
			return ArchiveInspection{}, err
		}
		inspection.ModelFile = modelFile
		inspection.ModelFileSize = modelFileSize
		return inspection, nil
	default:
		return inspection, nil
	}
}

func listTarArchiveFilesFromReader(path string, stream io.Reader) ([]tarArchiveFile, string, error) {
	reader, closeArchive, err := newClosableTarReader(path, stream)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = closeArchive() }()

	files := make([]tarArchiveFile, 0, 16)
	meaningfulRoots := make(map[string]struct{}, 2)
	hasArchiveRootFile := false
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		relative, keep, err := classifyArchiveEntry(header)
		if err != nil {
			return nil, "", err
		}
		if !keep {
			continue
		}
		files = append(files, tarArchiveFile{RelativePath: relative, SizeBytes: header.Size})

		root := archiveTopLevelRoot(relative)
		if root == "" {
			hasArchiveRootFile = true
			continue
		}
		meaningfulRoots[root] = struct{}{}
	}

	return files, deriveArchiveRootPrefix(meaningfulRoots, hasArchiveRootFile), nil
}

func classifyArchiveEntry(header *tar.Header) (string, bool, error) {
	relative, err := archiveRelativePath(header.Name)
	if err != nil {
		return "", false, err
	}
	switch header.Typeflag {
	case tar.TypeDir, tar.TypeXHeader, tar.TypeXGlobalHeader:
		return relative, false, nil
	case tar.TypeReg, tar.TypeRegA:
		return relative, true, nil
	case tar.TypeSymlink:
		return "", false, errors.New("refusing to inspect symbolic link tar entry")
	case tar.TypeLink:
		return "", false, errors.New("refusing to inspect hard link tar entry")
	default:
		return "", false, errors.New("refusing to inspect unsupported tar entry")
	}
}

func archiveTopLevelRoot(relative string) string {
	clean := strings.TrimSpace(filepath.ToSlash(relative))
	if clean == "" || clean == "." {
		return ""
	}
	parts := strings.Split(clean, "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func deriveArchiveRootPrefix(roots map[string]struct{}, hasArchiveRootFile bool) string {
	if hasArchiveRootFile || len(roots) != 1 {
		return ""
	}
	for root := range roots {
		return strings.TrimSpace(root)
	}
	return ""
}

func normalizedArchiveFilePath(relativePath string, rootPrefix string) (string, bool) {
	trimmed := strings.TrimSpace(filepath.ToSlash(relativePath))
	if trimmed == "" || trimmed == "." {
		return "", false
	}
	if rootPrefix == "" {
		return trimmed, true
	}
	prefix := strings.Trim(strings.TrimSpace(rootPrefix), "/") + "/"
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimPrefix(trimmed, prefix), true
	}
	return trimmed, true
}

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
