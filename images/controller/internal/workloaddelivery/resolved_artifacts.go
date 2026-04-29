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

package workloaddelivery

import (
	"encoding/json"
	"strings"
)

type ResolvedArtifact struct {
	ArtifactURI string
	Digest      string
	Family      string
	SizeBytes   int64
}

func ResolvedArtifactsFromAnnotations(annotations map[string]string) ([]ResolvedArtifact, bool, error) {
	if !IsSharedDirectResolvedDelivery(annotations) {
		return nil, false, nil
	}
	if models := strings.TrimSpace(annotations[ResolvedModelsAnnotation]); models != "" {
		artifacts, err := resolvedArtifactsFromModels(models)
		return artifacts, len(artifacts) > 0, err
	}

	return []ResolvedArtifact{{
		ArtifactURI: strings.TrimSpace(annotations[ResolvedArtifactURIAnnotation]),
		Digest:      strings.TrimSpace(annotations[ResolvedDigestAnnotation]),
		Family:      strings.TrimSpace(annotations[ResolvedArtifactFamilyAnnotation]),
	}}, true, nil
}

func IsSharedDirectResolvedDelivery(annotations map[string]string) bool {
	return strings.TrimSpace(annotations[ResolvedDeliveryModeAnnotation]) == DeliveryModeSharedDirect &&
		strings.TrimSpace(annotations[ResolvedDeliveryReasonAnnotation]) == DeliveryReasonNodeSharedRuntimePlane
}

type resolvedModelAnnotation struct {
	URI       string `json:"uri"`
	Digest    string `json:"digest"`
	Family    string `json:"family,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}

func resolvedArtifactsFromModels(value string) ([]ResolvedArtifact, error) {
	var entries []resolvedModelAnnotation
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return nil, err
	}
	artifacts := make([]ResolvedArtifact, 0, len(entries))
	for _, entry := range entries {
		artifacts = append(artifacts, ResolvedArtifact{
			ArtifactURI: strings.TrimSpace(entry.URI),
			Digest:      strings.TrimSpace(entry.Digest),
			Family:      strings.TrimSpace(entry.Family),
			SizeBytes:   entry.SizeBytes,
		})
	}
	return artifacts, nil
}
