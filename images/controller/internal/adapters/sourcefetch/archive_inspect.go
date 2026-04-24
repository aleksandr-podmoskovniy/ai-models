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
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
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
	case archiveio.IsTarArchive(path):
		return inspectTarModelArchive(path, requested)
	case archiveio.IsZipArchive(path):
		return inspectZipModelArchive(path, requested)
	default:
		return ArchiveInspection{}, errors.New("streaming archive inspection only supports tar/tar.gz/tgz/tar.zst/tar.zstd/tzst/zip")
	}
}

func InspectTarModelArchiveReader(path string, openReader openArchiveReaderFunc, requested modelsv1alpha1.ModelInputFormat) (ArchiveInspection, error) {
	if !archiveio.IsTarArchive(path) {
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
