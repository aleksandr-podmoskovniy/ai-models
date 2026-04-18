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

func describePublishLayers(ctx context.Context, layers []modelpackports.PublishLayer) ([]publishLayerDescriptor, error) {
	descriptors := make([]publishLayerDescriptor, 0, len(layers))
	targets := make(map[string]struct{}, len(layers))
	for index, layer := range layers {
		descriptor, err := describePublishLayer(ctx, layer)
		if err != nil {
			return nil, fmt.Errorf("invalid ModelPack layer %d: %w", index, err)
		}
		if _, exists := targets[descriptor.TargetPath]; exists {
			return nil, fmt.Errorf("duplicate ModelPack layer target path %q", descriptor.TargetPath)
		}
		targets[descriptor.TargetPath] = struct{}{}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
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
	diffHasher := sha256.New()
	blobHasher := sha256.New()
	counter := &countWriter{}
	compressedSink := io.MultiWriter(blobHasher, counter)

	archiveWriter, err := newArchiveWriter(compressedSink, layer.Compression)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	tarSink := io.MultiWriter(diffHasher, archiveWriter)
	if err := writeLayerArchive(tarSink, layer.SourcePath, layer.TargetPath, info); err != nil {
		_ = archiveWriter.Close()
		return publishLayerDescriptor{}, err
	}
	if err := archiveWriter.Close(); err != nil {
		return publishLayerDescriptor{}, err
	}

	return publishLayerDescriptor{
		Digest:      "sha256:" + hex.EncodeToString(blobHasher.Sum(nil)),
		DiffID:      "sha256:" + hex.EncodeToString(diffHasher.Sum(nil)),
		Size:        counter.n,
		MediaType:   mediaType,
		TargetPath:  filepath.ToSlash(strings.TrimSpace(layer.TargetPath)),
		Base:        layer.Base,
		Format:      layer.Format,
		Compression: normalizedArchiveCompression(layer.Compression),
	}, nil
}

func primaryModelPath(layers []publishLayerDescriptor) string {
	for _, layer := range layers {
		if isModelLayerBase(layer.Base) {
			return layer.TargetPath
		}
	}
	return materializedLayerPath
}
