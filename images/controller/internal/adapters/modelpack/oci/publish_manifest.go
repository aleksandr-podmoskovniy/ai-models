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

import "strings"

func buildConfigBlob(primaryPath string, descriptors []publishLayerDescriptor) ([]byte, error) {
	diffIDs := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		diffIDs = append(diffIDs, strings.TrimSpace(descriptor.DiffID))
	}
	return jsonMarshal(map[string]any{
		"descriptor": map[string]any{
			"name": strings.TrimSpace(primaryPath),
		},
		"modelfs": map[string]any{
			"type":    "layers",
			"diffIds": diffIDs,
		},
		"config": map[string]any{},
	})
}

func buildManifestBlob(configDescriptor blobDescriptor, layers []publishLayerDescriptor) ([]byte, error) {
	manifestLayers := make([]map[string]any, 0, len(layers))
	for _, layer := range layers {
		manifestLayers = append(manifestLayers, map[string]any{
			"mediaType": layer.MediaType,
			"digest":    layer.Digest,
			"size":      layer.Size,
			"annotations": map[string]string{
				ModelPackFilepathAnnotation: layer.TargetPath,
			},
		})
	}

	return jsonMarshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     ManifestMediaType,
		"artifactType":  ModelPackArtifactType,
		"config": map[string]any{
			"mediaType": ModelPackConfigMediaType,
			"digest":    configDescriptor.Digest,
			"size":      configDescriptor.Size,
		},
		"layers": manifestLayers,
	})
}
