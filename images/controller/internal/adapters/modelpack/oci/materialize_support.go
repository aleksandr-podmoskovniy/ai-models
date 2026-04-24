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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

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
	layers, err := decodeManifestLayers(manifest)
	if err != nil {
		return ""
	}
	candidates := make([]string, 0, len(layers))
	for _, layer := range layers {
		if !isModelLayerBase(layer.Base) {
			continue
		}
		target, resolveErr := resolvedModelLayerPath(destination, layer)
		if resolveErr != nil {
			return ""
		}
		if _, statErr := os.Stat(target); statErr != nil {
			return ""
		}
		candidates = append(candidates, target)
	}
	return commonModelRoot(candidates)
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

func resolvedModelLayerPath(destination string, layer publishLayerDescriptor) (string, error) {
	switch layer.Format {
	case modelpackports.LayerFormatRaw:
		target, err := materializeTargetPath(destination, layer.TargetPath)
		if err != nil {
			return "", err
		}
		parent := filepath.Dir(target)
		if parent == "." || parent == string(filepath.Separator) {
			return destination, nil
		}
		return parent, nil
	case modelpackports.LayerFormatTar:
		return materializeTargetPath(destination, layer.TargetPath)
	default:
		return "", fmt.Errorf("unsupported model layer format %q", layer.Format)
	}
}

func commonModelRoot(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	candidate := filepath.Clean(paths[0])
	for _, current := range paths[1:] {
		cleanCurrent := filepath.Clean(current)
		if cleanCurrent == candidate {
			continue
		}
		for {
			relative, err := filepath.Rel(candidate, cleanCurrent)
			if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				break
			}
			next := filepath.Dir(candidate)
			if next == candidate || next == "." {
				return ""
			}
			candidate = next
		}
	}
	return candidate
}

func digestFromOCIReference(reference string) string {
	_, digest, found := strings.Cut(strings.TrimSpace(reference), "@")
	if !found {
		return ""
	}
	return strings.TrimSpace(digest)
}
