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

package kitops

import (
	"errors"
	"fmt"
	"strings"
)

func validateModelPackPayload(payload map[string]any) error {
	manifest, _ := payload["manifest"].(map[string]any)
	if manifest == nil {
		return errors.New("registry inspect payload is missing manifest")
	}
	if err := validateModelPackManifest(manifest); err != nil {
		return err
	}
	layers, _ := manifest["layers"].([]any)
	if err := validateModelPackLayers(layers); err != nil {
		return err
	}
	configBlob, _ := payload["configBlob"].(map[string]any)
	if err := validateModelPackConfigBlob(configBlob, len(layers)); err != nil {
		return err
	}
	return nil
}

func validateModelPackManifest(manifest map[string]any) error {
	if got := parseInt(manifest["schemaVersion"]); got != 2 {
		return fmt.Errorf("registry manifest schemaVersion must be 2, got %d", got)
	}
	if got := strings.TrimSpace(stringValue(manifest["artifactType"])); got != modelPackArtifactType {
		return fmt.Errorf("registry manifest artifactType must be %q, got %q", modelPackArtifactType, got)
	}

	configDescriptor, _ := manifest["config"].(map[string]any)
	if configDescriptor == nil {
		return errors.New("registry manifest is missing config descriptor")
	}
	if got := strings.TrimSpace(stringValue(configDescriptor["mediaType"])); got != modelPackConfigMediaType {
		return fmt.Errorf("registry manifest config mediaType must be %q, got %q", modelPackConfigMediaType, got)
	}
	if strings.TrimSpace(stringValue(configDescriptor["digest"])) == "" {
		return errors.New("registry manifest config descriptor is missing digest")
	}
	return nil
}

func validateModelPackLayers(layers []any) error {
	if len(layers) == 0 {
		return errors.New("registry manifest must contain at least one layer")
	}
	for index, layer := range layers {
		layerMap, _ := layer.(map[string]any)
		if layerMap == nil {
			return fmt.Errorf("registry manifest layer %d is invalid", index)
		}
		if got := strings.TrimSpace(stringValue(layerMap["mediaType"])); got != modelPackWeightLayerType {
			return fmt.Errorf("registry manifest layer %d mediaType must be %q, got %q", index, modelPackWeightLayerType, got)
		}
		annotations, _ := layerMap["annotations"].(map[string]any)
		if strings.TrimSpace(stringValue(annotations[modelPackFilepathAnnotation])) == "" {
			return fmt.Errorf("registry manifest layer %d is missing %q annotation", index, modelPackFilepathAnnotation)
		}
		if strings.TrimSpace(stringValue(layerMap["digest"])) == "" {
			return fmt.Errorf("registry manifest layer %d is missing digest", index)
		}
	}
	return nil
}

func validateModelPackConfigBlob(configBlob map[string]any, layerCount int) error {
	if configBlob == nil {
		return errors.New("registry inspect payload is missing config blob")
	}
	descriptor, _ := configBlob["descriptor"].(map[string]any)
	if descriptor == nil {
		return errors.New("registry config blob is missing descriptor")
	}
	modelfs, _ := configBlob["modelfs"].(map[string]any)
	if modelfs == nil {
		return errors.New("registry config blob is missing modelfs")
	}
	if got := strings.TrimSpace(stringValue(modelfs["type"])); got != "layers" {
		return fmt.Errorf("registry config blob modelfs.type must be %q, got %q", "layers", got)
	}
	diffIDs, _ := modelfs["diffIds"].([]any)
	if len(diffIDs) == 0 {
		return errors.New("registry config blob modelfs.diffIds must not be empty")
	}
	if len(diffIDs) != layerCount {
		return fmt.Errorf("registry config blob diffIds count %d must match layer count %d", len(diffIDs), layerCount)
	}
	for index, diffID := range diffIDs {
		if strings.TrimSpace(stringValue(diffID)) == "" {
			return fmt.Errorf("registry config blob modelfs.diffIds[%d] must not be empty", index)
		}
	}
	return nil
}

func artifactDigestFromInspectPayload(payload map[string]any) string {
	value, _ := payload["digest"].(string)
	return strings.TrimSpace(value)
}

func artifactMediaTypeFromInspectPayload(payload map[string]any) string {
	manifest, _ := payload["manifest"].(map[string]any)
	if manifest != nil {
		if artifactType, _ := manifest["artifactType"].(string); strings.TrimSpace(artifactType) != "" {
			return strings.TrimSpace(artifactType)
		}
	}
	return ""
}

func inspectModelPackSize(payload map[string]any) int64 {
	var total int64
	manifest, _ := payload["manifest"].(map[string]any)
	if manifest == nil {
		return 0
	}

	if config, _ := manifest["config"].(map[string]any); config != nil {
		total += parseSize(config["size"])
	}
	if layers, _ := manifest["layers"].([]any); layers != nil {
		for _, layer := range layers {
			layerMap, _ := layer.(map[string]any)
			total += parseSize(layerMap["size"])
		}
	}

	return total
}

func parseSize(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case float64:
		return int64(typed)
	case int:
		return int64(typed)
	default:
		return 0
	}
}

func stringValue(value any) string {
	stringed, _ := value.(string)
	return stringed
}

func parseInt(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
