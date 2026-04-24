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

package oci

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func defaultPublishLayers(input modelpackports.PublishInput) ([]modelpackports.PublishLayer, error) {
	if len(input.Layers) > 0 {
		return input.Layers, nil
	}
	if strings.TrimSpace(input.ModelDir) == "" {
		return nil, errors.New("either model directory or explicit ModelPack layers must be provided")
	}
	return []modelpackports.PublishLayer{
		{
			SourcePath:  input.ModelDir,
			TargetPath:  materializedLayerPath,
			Base:        modelpackports.LayerBaseModel,
			Format:      modelpackports.LayerFormatTar,
			Compression: modelpackports.LayerCompressionNone,
		},
	}, nil
}

func planPublishLayers(layers []modelpackports.PublishLayer) ([]publishLayerDescriptor, error) {
	plans := make([]publishLayerDescriptor, 0, len(layers))
	targets := make(map[string]struct{}, len(layers))
	for index, layer := range layers {
		plan, err := planPublishLayer(layer)
		if err != nil {
			return nil, fmt.Errorf("invalid ModelPack layer %d: %w", index, err)
		}
		if _, exists := targets[plan.TargetPath]; exists {
			return nil, fmt.Errorf("duplicate ModelPack layer target path %q", plan.TargetPath)
		}
		targets[plan.TargetPath] = struct{}{}
		plans = append(plans, plan)
	}
	return plans, nil
}

func planPublishLayer(layer modelpackports.PublishLayer) (publishLayerDescriptor, error) {
	targetPath, mediaType, err := plannedPublishLayerIdentity(layer)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	if err := validatePlannedPublishSource(layer); err != nil {
		return publishLayerDescriptor{}, err
	}
	return publishLayerDescriptor{
		MediaType:   mediaType,
		TargetPath:  targetPath,
		Base:        layer.Base,
		Format:      layer.Format,
		Compression: plannedPublishLayerCompression(layer),
	}, nil
}

func plannedPublishLayerIdentity(layer modelpackports.PublishLayer) (string, string, error) {
	if strings.TrimSpace(layer.TargetPath) == "" {
		return "", "", errors.New("target path must not be empty")
	}
	if err := validatePublishLayerBase(layer.Base); err != nil {
		return "", "", err
	}
	if err := validatePublishLayerFormat(layer.Format); err != nil {
		return "", "", err
	}
	if err := validatePublishLayerCompression(layer.Compression); err != nil {
		return "", "", err
	}
	mediaType, err := buildLayerMediaType(layer.Base, layer.Format, layer.Compression)
	if err != nil {
		return "", "", err
	}
	if strings.Contains(strings.TrimSpace(layer.TargetPath), `\`) {
		return "", "", fmt.Errorf("target path %q must use slash separators", layer.TargetPath)
	}
	return filepath.ToSlash(strings.TrimSpace(layer.TargetPath)), mediaType, nil
}

func validatePlannedPublishSource(layer modelpackports.PublishLayer) error {
	switch {
	case layer.ObjectSource != nil:
		return validateObjectSourceLayer(layer)
	case layer.Archive != nil:
		return validateArchiveSourceLayer(layer)
	case strings.TrimSpace(layer.SourcePath) == "":
		return errors.New("source path must not be empty")
	case layer.Format == modelpackports.LayerFormatRaw &&
		layer.Compression != "" &&
		layer.Compression != modelpackports.LayerCompressionNone:
		return fmt.Errorf("raw ModelPack layer %q must not declare compression", layer.SourcePath)
	default:
		return nil
	}
}

func plannedPublishLayerCompression(layer modelpackports.PublishLayer) modelpackports.LayerCompression {
	if layer.Format == modelpackports.LayerFormatRaw {
		return modelpackports.LayerCompressionNone
	}
	return normalizedArchiveCompression(layer.Compression)
}

func describePublishLayer(ctx context.Context, layer modelpackports.PublishLayer) (publishLayerDescriptor, error) {
	if strings.TrimSpace(layer.TargetPath) == "" {
		return publishLayerDescriptor{}, errors.New("target path must not be empty")
	}
	if err := validatePublishLayerBase(layer.Base); err != nil {
		return publishLayerDescriptor{}, err
	}
	if err := validatePublishLayerFormat(layer.Format); err != nil {
		return publishLayerDescriptor{}, err
	}
	if err := validatePublishLayerCompression(layer.Compression); err != nil {
		return publishLayerDescriptor{}, err
	}
	mediaType, err := buildLayerMediaType(layer.Base, layer.Format, layer.Compression)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	if strings.Contains(strings.TrimSpace(layer.TargetPath), `\`) {
		return publishLayerDescriptor{}, fmt.Errorf("target path %q must use slash separators", layer.TargetPath)
	}
	if layer.ObjectSource != nil {
		return describeObjectSourcePublishLayer(ctx, layer, mediaType)
	}
	if layer.Archive != nil {
		return describeArchiveSourcePublishLayer(ctx, layer, mediaType)
	}
	if strings.TrimSpace(layer.SourcePath) == "" {
		return publishLayerDescriptor{}, errors.New("source path must not be empty")
	}

	info, err := os.Stat(layer.SourcePath)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	if layer.Format == modelpackports.LayerFormatRaw {
		return describeRawPublishLayer(layer, info, mediaType)
	}
	return describeArchivePublishLayer(layer, info, mediaType)
}

func describeRawPublishLayer(
	layer modelpackports.PublishLayer,
	info os.FileInfo,
	mediaType string,
) (publishLayerDescriptor, error) {
	if info.IsDir() {
		return publishLayerDescriptor{}, fmt.Errorf("raw ModelPack layer %q must point to a regular file", layer.SourcePath)
	}
	if layer.Compression != "" && layer.Compression != modelpackports.LayerCompressionNone {
		return publishLayerDescriptor{}, fmt.Errorf("raw ModelPack layer %q must not declare compression", layer.SourcePath)
	}

	stream, err := os.Open(layer.SourcePath)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	defer stream.Close()

	hasher := sha256.New()
	counter := &countWriter{}
	if _, err := io.Copy(io.MultiWriter(hasher, counter), stream); err != nil {
		return publishLayerDescriptor{}, err
	}
	digest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	return publishLayerDescriptor{
		Digest:      digest,
		DiffID:      digest,
		Size:        counter.n,
		MediaType:   mediaType,
		TargetPath:  filepath.ToSlash(strings.TrimSpace(layer.TargetPath)),
		Base:        layer.Base,
		Format:      layer.Format,
		Compression: modelpackports.LayerCompressionNone,
	}, nil
}

func describeArchivePublishLayer(
	layer modelpackports.PublishLayer,
	info os.FileInfo,
	mediaType string,
) (publishLayerDescriptor, error) {
	return describeGeneratedArchiveLayer(layer, mediaType, func(writer io.Writer) error {
		return writeLayerArchive(writer, layer.SourcePath, layer.TargetPath, info)
	})
}

func publishedModelPath(layers []publishLayerDescriptor) string {
	if root := publishedModelRoot(layers, modelpackports.LayerBaseModel); root != "" {
		return root
	}
	if root := publishedModelRoot(layers, modelpackports.LayerBaseModelConfig); root != "" {
		return root
	}
	return materializedLayerPath
}

func publishedModelRoot(layers []publishLayerDescriptor, base modelpackports.LayerBase) string {
	candidates := make([]string, 0, len(layers))
	for _, layer := range layers {
		if layer.Base != base {
			continue
		}
		candidate := publishedModelPathCandidate(layer)
		if candidate == "" {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return ""
	}
	return normalizePublishedModelRoot(commonPublishedPath(candidates))
}

func publishedModelPathCandidate(layer publishLayerDescriptor) string {
	target := cleanPublishedPath(layer.TargetPath)
	if target == "" {
		return ""
	}
	if layer.Base == modelpackports.LayerBaseModelConfig && layer.Format == modelpackports.LayerFormatRaw {
		return cleanPublishedPath(path.Dir(target))
	}
	return target
}

func commonPublishedPath(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	candidate := cleanPublishedPath(candidates[0])
	for _, current := range candidates[1:] {
		cleanCurrent := cleanPublishedPath(current)
		for !publishedPathContains(candidate, cleanCurrent) {
			next := cleanPublishedPath(path.Dir(candidate))
			if next == candidate || next == "" {
				return ""
			}
			candidate = next
		}
	}
	return candidate
}

func cleanPublishedPath(value string) string {
	clean := path.Clean(strings.Trim(strings.TrimSpace(value), "/"))
	switch clean {
	case "", "/":
		return "."
	default:
		return clean
	}
}

func publishedPathContains(base, target string) bool {
	base = cleanPublishedPath(base)
	target = cleanPublishedPath(target)
	if base == "." {
		return true
	}
	return target == base || strings.HasPrefix(target, base+"/")
}

func normalizePublishedModelRoot(value string) string {
	switch cleanPublishedPath(value) {
	case "", ".":
		return materializedLayerPath
	default:
		return cleanPublishedPath(value)
	}
}
