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
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

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
	for index, layer := range layers {
		layerMap, _ := layer.(map[string]any)
		if layerMap == nil {
			return fmt.Errorf("registry manifest layer %d is invalid", index)
		}
		digest := strings.TrimSpace(stringValue(layerMap["digest"]))
		if digest == "" {
			return fmt.Errorf("registry manifest layer %d is missing digest", index)
		}

		resp, err := GetBlobResponse(ctx, client, reference, digest, auth)
		if err != nil {
			return err
		}
		if err := extractTarLayer(resp.Body, destination); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to extract ModelPack layer %d: %w", index, err)
		}
		if err := resp.Body.Close(); err != nil {
			return err
		}
	}
	return nil
}

func extractTarLayer(stream io.Reader, destination string) error {
	tarReader, err := newTarReader(stream)
	if err != nil {
		return err
	}

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		target, err := archiveTargetPath(destination, header.Name)
		if err != nil {
			return err
		}
		if err := extractTarEntry(tarReader, header, target); err != nil {
			return err
		}
	}
}

func newTarReader(stream io.Reader) (*tar.Reader, error) {
	buffered := bufio.NewReader(stream)
	header, err := buffered.Peek(2)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(header) == 2 && header[0] == 0x1f && header[1] == 0x8b {
		gzipReader, err := gzip.NewReader(buffered)
		if err != nil {
			return nil, err
		}
		return tar.NewReader(gzipReader), nil
	}
	return tar.NewReader(buffered), nil
}

func extractTarEntry(reader *tar.Reader, header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		return writeExtractedFile(target, reader)
	case tar.TypeSymlink:
		return fmt.Errorf("refusing to extract symbolic link tar entry %q", header.Name)
	case tar.TypeLink:
		return fmt.Errorf("refusing to extract hard link tar entry %q", header.Name)
	default:
		return fmt.Errorf("refusing to extract unsupported tar entry %q", header.Name)
	}
}

func writeExtractedFile(target string, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	stream, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(stream, reader); err != nil {
		stream.Close()
		return err
	}
	return stream.Close()
}

func archiveTargetPath(destination, name string) (string, error) {
	relative, err := archiveRelativePath(name)
	if err != nil {
		return "", err
	}
	if relative == "." {
		return destination, nil
	}
	return filepath.Join(destination, relative), nil
}

func archiveRelativePath(name string) (string, error) {
	rawName := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if rawName == "" {
		return "", errors.New("archive entry name must not be empty")
	}

	parts := strings.Split(rawName, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("refusing to extract archive entry outside of destination: %q", name)
		default:
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return ".", nil
	}
	return filepath.Join(result...), nil
}

func resolveModelPath(destination string, payload InspectPayload) (string, error) {
	manifest, _ := payload["manifest"].(map[string]any)
	if modelPath := manifestModelPath(destination, manifest); modelPath != "" {
		return modelPath, nil
	}

	configBlob, _ := payload["configBlob"].(map[string]any)
	if modelPath := descriptorModelPath(destination, configBlob); modelPath != "" {
		return modelPath, nil
	}

	return normalizeExtractedRoot(destination)
}

func manifestModelPath(destination string, manifest map[string]any) string {
	layers, _ := manifest["layers"].([]any)
	var candidate string
	for _, layer := range layers {
		layerMap, _ := layer.(map[string]any)
		annotations, _ := layerMap["annotations"].(map[string]any)
		filePath := strings.TrimSpace(stringValue(annotations[ModelPackFilepathAnnotation]))
		if filePath == "" {
			continue
		}
		target := filepath.Join(destination, filePath)
		if _, err := os.Stat(target); err != nil {
			return ""
		}
		if candidate == "" {
			candidate = target
			continue
		}
		if candidate != target {
			return ""
		}
	}
	return candidate
}

func descriptorModelPath(destination string, configBlob map[string]any) string {
	descriptor, _ := configBlob["descriptor"].(map[string]any)
	name := strings.TrimSpace(stringValue(descriptor["name"]))
	if name == "" {
		return ""
	}
	target := filepath.Join(destination, name)
	if _, err := os.Stat(target); err != nil {
		return ""
	}
	return target
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
	if len(meaningful) == 1 {
		return filepath.Join(destination, meaningful[0].Name()), nil
	}
	return destination, nil
}

func digestFromOCIReference(reference string) string {
	_, digest, found := strings.Cut(strings.TrimSpace(reference), "@")
	if !found {
		return ""
	}
	return strings.TrimSpace(digest)
}
