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
	"io"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
)

func inspectZipModelArchive(path string, requested modelsv1alpha1.ModelInputFormat) (ArchiveInspection, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return ArchiveInspection{}, err
	}
	defer archive.Close()

	return inspectZipArchiveFiles(archive.File, requested)
}

func inspectZipArchiveFiles(files []*zip.File, requested modelsv1alpha1.ModelInputFormat) (ArchiveInspection, error) {
	archiveFiles := make([]tarArchiveFile, 0, len(files))
	meaningfulRoots := make(map[string]struct{}, 2)
	hasArchiveRootFile := false
	for _, file := range files {
		relative, keep, err := classifyZipArchiveEntry(file)
		if err != nil {
			return ArchiveInspection{}, err
		}
		if !keep {
			continue
		}
		archiveFiles = append(archiveFiles, tarArchiveFile{RelativePath: relative, SizeBytes: int64(file.UncompressedSize64)})

		root := archiveTopLevelRoot(relative)
		if root == "" {
			hasArchiveRootFile = true
			continue
		}
		meaningfulRoots[root] = struct{}{}
	}

	rootPrefix := deriveArchiveRootPrefix(meaningfulRoots, hasArchiveRootFile)
	inspection, err := prepareArchiveInspection(archiveFiles, rootPrefix, requested)
	if err != nil {
		return ArchiveInspection{}, err
	}

	switch inspection.InputFormat {
	case modelsv1alpha1.ModelInputFormatDiffusers:
		modelIndexPayload, weightStats, err := summarizeZipDiffusersArchive(files, rootPrefix, inspection.SelectedFiles)
		if err != nil {
			return ArchiveInspection{}, err
		}
		inspection.ModelIndexPayload = modelIndexPayload
		inspection.WeightStats = weightStats
		return inspection, nil
	case modelsv1alpha1.ModelInputFormatSafetensors:
		configPayload, weightStats, err := summarizeZipSafetensorsArchive(files, rootPrefix, inspection.SelectedFiles)
		if err != nil {
			return ArchiveInspection{}, err
		}
		inspection.ConfigPayload = configPayload
		inspection.WeightStats = weightStats
		return inspection, nil
	case modelsv1alpha1.ModelInputFormatGGUF:
		modelFile, modelFileSize, err := summarizeGGUFArchive(archiveFiles, rootPrefix, inspection.SelectedFiles)
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

func classifyZipArchiveEntry(file *zip.File) (string, bool, error) {
	relative, err := archiveio.RelativePath(file.Name)
	if err != nil {
		return "", false, err
	}
	if file.FileInfo().IsDir() {
		return relative, false, nil
	}
	if archiveio.IsZipSymlink(file) {
		return "", false, errors.New("refusing to inspect symbolic link zip entry")
	}
	return relative, true, nil
}

func summarizeZipSafetensorsArchive(files []*zip.File, rootPrefix string, selectedFiles []string) ([]byte, WeightStats, error) {
	return summarizeZipSafetensorsLikeArchive(files, rootPrefix, selectedFiles, "config.json", "safetensors", isSafetensorsWeightFile, "safetensors")
}

func summarizeZipDiffusersArchive(files []*zip.File, rootPrefix string, selectedFiles []string) ([]byte, WeightStats, error) {
	return summarizeZipSafetensorsLikeArchive(files, rootPrefix, selectedFiles, "model_index.json", "diffusers", modelformat.IsDiffusersWeightFile, "diffusers weight")
}

func summarizeZipSafetensorsLikeArchive(
	files []*zip.File,
	rootPrefix string,
	selectedFiles []string,
	configPath string,
	label string,
	isWeightFile func(string) bool,
	weightLabel string,
) ([]byte, WeightStats, error) {
	selected := archiveFileSet(selectedFiles)

	var (
		configPayload []byte
		weightStats   WeightStats
	)
	for _, file := range files {
		relative, keep, err := classifyZipArchiveEntry(file)
		if err != nil {
			return nil, WeightStats{}, err
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
		case normalized == configPath:
			stream, err := file.Open()
			if err != nil {
				return nil, WeightStats{}, err
			}
			configPayload, err = io.ReadAll(stream)
			_ = stream.Close()
			if err != nil {
				return nil, WeightStats{}, err
			}
		case isWeightFile(normalized):
			weightStats.add(int64(file.UncompressedSize64))
		}
	}

	if len(configPayload) == 0 {
		return nil, WeightStats{}, errors.New("zip " + label + " summary requires " + configPath + " in selected files")
	}
	if weightStats.TotalBytes <= 0 {
		return nil, WeightStats{}, errors.New("zip " + label + " summary requires positive " + weightLabel + " bytes")
	}
	return configPayload, weightStats, nil
}
