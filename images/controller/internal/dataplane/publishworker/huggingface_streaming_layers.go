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

package publishworker

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const (
	objectSourceBundleMaxFileBytes  int64 = 16 << 20
	objectSourceBundleMaxLayerBytes int64 = 64 << 20
)

func buildObjectSourcePublishLayers(
	sourcePath string,
	reader modelpackports.PublishObjectReader,
	files []modelpackports.PublishObjectFile,
) ([]modelpackports.PublishLayer, error) {
	if reader == nil {
		return nil, fmt.Errorf("object source reader must not be nil")
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("object source files must not be empty")
	}

	if len(files) == 1 {
		return buildSingleObjectSourcePublishLayer(sourcePath, reader, files[0])
	}

	bundledFiles, rawFiles, err := partitionObjectSourceFilesForPublish(files)
	if err != nil {
		return nil, err
	}

	layers := make([]modelpackports.PublishLayer, 0, len(rawFiles)+1)
	if len(bundledFiles) > 0 {
		layers = append(layers, modelpackports.PublishLayer{
			SourcePath:  strings.TrimSpace(sourcePath),
			TargetPath:  modelpackports.MaterializedModelPathName,
			Base:        modelpackports.LayerBaseModel,
			Format:      modelpackports.LayerFormatTar,
			Compression: modelpackports.LayerCompressionNone,
			ObjectSource: &modelpackports.PublishObjectSource{
				Reader: reader,
				Files:  bundledFiles,
			},
		})
	}

	for _, file := range rawFiles {
		targetPath, err := rawObjectSourceTargetPath(file.TargetPath, false)
		if err != nil {
			return nil, err
		}
		layers = append(layers, rawObjectSourcePublishLayer(sourcePath, reader, file, targetPath))
	}
	return layers, nil
}

func buildSingleObjectSourcePublishLayer(
	sourcePath string,
	reader modelpackports.PublishObjectReader,
	file modelpackports.PublishObjectFile,
) ([]modelpackports.PublishLayer, error) {
	targetPath, err := rawObjectSourceTargetPath(file.TargetPath, true)
	if err != nil {
		return nil, err
	}
	return []modelpackports.PublishLayer{
		rawObjectSourcePublishLayer(sourcePath, reader, file, targetPath),
	}, nil
}

func rawObjectSourcePublishLayer(
	sourcePath string,
	reader modelpackports.PublishObjectReader,
	file modelpackports.PublishObjectFile,
	targetPath string,
) modelpackports.PublishLayer {
	return modelpackports.PublishLayer{
		SourcePath:  strings.TrimSpace(sourcePath),
		TargetPath:  targetPath,
		Base:        publishLayerBaseForModelFile(targetPath),
		Format:      modelpackports.LayerFormatRaw,
		Compression: modelpackports.LayerCompressionNone,
		ObjectSource: &modelpackports.PublishObjectSource{
			Reader: reader,
			Files: []modelpackports.PublishObjectFile{
				{
					SourcePath: strings.TrimSpace(file.SourcePath),
					TargetPath: targetPath,
					SizeBytes:  file.SizeBytes,
					ETag:       strings.TrimSpace(file.ETag),
				},
			},
		},
	}
}

func partitionObjectSourceFilesForPublish(files []modelpackports.PublishObjectFile) ([]modelpackports.PublishObjectFile, []modelpackports.PublishObjectFile, error) {
	bundled := make([]modelpackports.PublishObjectFile, 0, len(files))
	raw := make([]modelpackports.PublishObjectFile, 0, len(files))
	var bundledBytes int64

	for _, file := range files {
		cleanTarget, err := cleanMirrorRelativePath(file.TargetPath)
		if err != nil {
			return nil, nil, err
		}

		normalized := modelpackports.PublishObjectFile{
			SourcePath: strings.TrimSpace(file.SourcePath),
			TargetPath: cleanTarget,
			SizeBytes:  file.SizeBytes,
			ETag:       strings.TrimSpace(file.ETag),
		}
		if shouldBundleObjectSourceFile(normalized, bundledBytes) {
			bundled = append(bundled, normalized)
			bundledBytes += normalized.SizeBytes
			continue
		}
		raw = append(raw, normalized)
	}

	return bundled, raw, nil
}

func shouldBundleObjectSourceFile(file modelpackports.PublishObjectFile, bundledBytes int64) bool {
	if file.SizeBytes <= 0 {
		return false
	}
	if looksLikeLargeModelPayload(file.TargetPath) {
		return false
	}
	if file.SizeBytes > objectSourceBundleMaxFileBytes {
		return false
	}
	return bundledBytes+file.SizeBytes <= objectSourceBundleMaxLayerBytes
}

func rawObjectSourceTargetPath(targetPath string, singleFile bool) (string, error) {
	cleanTarget, err := cleanMirrorRelativePath(targetPath)
	if err != nil {
		return "", err
	}
	if singleFile {
		return cleanTarget, nil
	}
	return path.Join(modelpackports.MaterializedModelPathName, cleanTarget), nil
}

func publishLayerBaseForModelFile(targetPath string) modelpackports.LayerBase {
	if strings.EqualFold(filepath.Base(strings.TrimSpace(targetPath)), "config.json") {
		return modelpackports.LayerBaseModelConfig
	}
	return modelpackports.LayerBaseModel
}

func looksLikeLargeModelPayload(targetPath string) bool {
	lowerTarget := strings.ToLower(strings.TrimSpace(targetPath))
	switch {
	case strings.HasSuffix(lowerTarget, ".safetensors"),
		strings.HasSuffix(lowerTarget, ".gguf"),
		strings.HasSuffix(lowerTarget, ".bin"),
		strings.HasSuffix(lowerTarget, ".onnx"),
		strings.HasSuffix(lowerTarget, ".pt"),
		strings.HasSuffix(lowerTarget, ".pth"),
		strings.HasSuffix(lowerTarget, ".ckpt"),
		strings.HasSuffix(lowerTarget, ".pb"),
		strings.HasSuffix(lowerTarget, ".tflite"):
		return true
	default:
		return false
	}
}
