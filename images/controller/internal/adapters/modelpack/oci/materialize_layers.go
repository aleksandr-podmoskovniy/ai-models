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
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
)

func decodeManifestLayers(manifest map[string]any) ([]publishLayerDescriptor, error) {
	layers, _ := manifest["layers"].([]any)
	layerSet, err := classifyManifestLayers(layers)
	if err != nil {
		return nil, err
	}
	if layerSet.Chunked() {
		return nil, errors.New("chunked ModelPack layout is not a legacy layer layout")
	}
	return layerSet.Legacy, nil
}

func decodeLegacyManifestLayer(index int, layerMap map[string]any) (publishLayerDescriptor, error) {
	mediaType := strings.TrimSpace(stringValue(layerMap["mediaType"]))
	parsedType, err := parseLayerMediaType(mediaType)
	if err != nil {
		return publishLayerDescriptor{}, fmt.Errorf("registry manifest layer %d mediaType is invalid: %w", index, err)
	}
	annotations, _ := layerMap["annotations"].(map[string]any)
	targetPath := strings.TrimSpace(stringValue(annotations[ModelPackFilepathAnnotation]))
	if targetPath == "" {
		return publishLayerDescriptor{}, fmt.Errorf("registry manifest layer %d is missing %q annotation", index, ModelPackFilepathAnnotation)
	}
	digest := strings.TrimSpace(stringValue(layerMap["digest"]))
	if digest == "" {
		return publishLayerDescriptor{}, fmt.Errorf("registry manifest layer %d is missing digest", index)
	}
	return publishLayerDescriptor{
		Digest:      digest,
		Size:        parseSize(layerMap["size"]),
		MediaType:   mediaType,
		TargetPath:  targetPath,
		Base:        parsedType.Base,
		Format:      parsedType.Format,
		Compression: parsedType.Compression,
	}, nil
}

func extractLayers(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	payload InspectPayload,
	destination string,
) error {
	manifest, _ := payload["manifest"].(map[string]any)
	layers, _ := manifest["layers"].([]any)
	layerSet, err := classifyManifestLayers(layers)
	if err != nil {
		return err
	}
	for index, layer := range layerSet.Legacy {
		resp, err := GetBlobResponse(ctx, client, reference, layer.Digest, auth)
		if err != nil {
			return err
		}
		if err := extractManifestLayer(resp.Body, destination, layer); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to extract ModelPack layer %d: %w", index, err)
		}
		if err := resp.Body.Close(); err != nil {
			return err
		}
	}
	if layerSet.Chunked() {
		return extractChunkedLayers(ctx, client, reference, auth, destination, layerSet)
	}
	return nil
}

func extractManifestLayer(stream io.Reader, destination string, layer publishLayerDescriptor) error {
	switch layer.Format {
	case modelpackports.LayerFormatRaw:
		return extractRawLayer(stream, destination, layer.TargetPath)
	case modelpackports.LayerFormatTar:
		return extractArchiveLayer(stream, destination, layer.TargetPath, layer.Compression)
	default:
		return fmt.Errorf("unsupported materialization format %q", layer.Format)
	}
}

func extractRawLayer(stream io.Reader, destination, targetPath string) error {
	target, err := materializeTargetPath(destination, targetPath)
	if err != nil {
		return err
	}
	return archiveio.WriteExtractedFile(target, stream)
}

func extractArchiveLayer(
	stream io.Reader,
	destination string,
	targetPath string,
	compression modelpackports.LayerCompression,
) error {
	reader, err := newArchiveReader(stream, compression)
	if err != nil {
		return err
	}
	defer reader.Close()

	root, err := materializeTargetPath(destination, filepath.Dir(strings.TrimSpace(targetPath)))
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(reader)
	return extractTar(tarReader, root)
}

func newArchiveReader(stream io.Reader, compression modelpackports.LayerCompression) (io.ReadCloser, error) {
	switch compression {
	case modelpackports.LayerCompressionNone:
		return io.NopCloser(stream), nil
	case modelpackports.LayerCompressionGzip, modelpackports.LayerCompressionGzipFastest:
		return gzip.NewReader(stream)
	case modelpackports.LayerCompressionZstd:
		decoder, err := zstd.NewReader(stream)
		if err != nil {
			return nil, err
		}
		return zstdReadCloser{Decoder: decoder}, nil
	default:
		return nil, fmt.Errorf("unsupported archive compression %q", compression)
	}
}

type zstdReadCloser struct {
	*zstd.Decoder
}

func (r zstdReadCloser) Close() error {
	r.Decoder.Close()
	return nil
}

func extractTar(tr *tar.Reader, extractDir string) error {
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		target, err := archiveio.TargetPath(extractDir, header.Name)
		if err != nil {
			return err
		}
		if err := archiveio.ExtractTarEntry(tr, header, target); err != nil {
			return err
		}
	}
}

func materializeTargetPath(destination, layerPath string) (string, error) {
	relative, err := archiveio.RelativePath(layerPath)
	if err != nil {
		return "", err
	}
	if relative == "." {
		return destination, nil
	}
	return filepath.Join(destination, relative), nil
}
