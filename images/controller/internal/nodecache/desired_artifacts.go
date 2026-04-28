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

package nodecache

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	deliverycontract "github.com/deckhouse/ai-models/controller/internal/workloaddelivery"
)

const (
	WorkloadResolvedDigestAnnotation         = deliverycontract.ResolvedDigestAnnotation
	WorkloadResolvedArtifactURIAnnotation    = deliverycontract.ResolvedArtifactURIAnnotation
	WorkloadResolvedArtifactFamilyAnnotation = deliverycontract.ResolvedArtifactFamilyAnnotation
	WorkloadResolvedDeliveryModeAnnotation   = deliverycontract.ResolvedDeliveryModeAnnotation
	WorkloadResolvedDeliveryReasonAnnotation = deliverycontract.ResolvedDeliveryReasonAnnotation
	WorkloadResolvedModelsAnnotation         = deliverycontract.ResolvedModelsAnnotation

	WorkloadDeliveryModeSharedDirect       = deliverycontract.DeliveryModeSharedDirect
	WorkloadDeliveryReasonNodeCacheRuntime = deliverycontract.DeliveryReasonNodeSharedRuntimePlane
)

type DesiredArtifact struct {
	ArtifactURI string
	Digest      string
	Family      string
	SizeBytes   int64
}

func DesiredArtifactsFromWorkloadAnnotations(annotations map[string]string) ([]DesiredArtifact, bool, error) {
	if strings.TrimSpace(annotations[WorkloadResolvedDeliveryModeAnnotation]) != WorkloadDeliveryModeSharedDirect ||
		strings.TrimSpace(annotations[WorkloadResolvedDeliveryReasonAnnotation]) != WorkloadDeliveryReasonNodeCacheRuntime {
		return nil, false, nil
	}
	if models := strings.TrimSpace(annotations[WorkloadResolvedModelsAnnotation]); models != "" {
		artifacts, err := desiredArtifactsFromResolvedModels(models)
		return artifacts, len(artifacts) > 0, err
	}

	artifact := DesiredArtifact{
		ArtifactURI: strings.TrimSpace(annotations[WorkloadResolvedArtifactURIAnnotation]),
		Digest:      strings.TrimSpace(annotations[WorkloadResolvedDigestAnnotation]),
		Family:      strings.TrimSpace(annotations[WorkloadResolvedArtifactFamilyAnnotation]),
	}
	artifacts, err := NormalizeDesiredArtifacts([]DesiredArtifact{artifact})
	return artifacts, len(artifacts) > 0, err
}

type resolvedModelAnnotation struct {
	URI       string `json:"uri"`
	Digest    string `json:"digest"`
	Family    string `json:"family,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}

func desiredArtifactsFromResolvedModels(value string) ([]DesiredArtifact, error) {
	var entries []resolvedModelAnnotation
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return nil, err
	}
	artifacts := make([]DesiredArtifact, 0, len(entries))
	for _, entry := range entries {
		artifacts = append(artifacts, DesiredArtifact{
			ArtifactURI: entry.URI,
			Digest:      entry.Digest,
			Family:      entry.Family,
			SizeBytes:   entry.SizeBytes,
		})
	}
	return NormalizeDesiredArtifacts(artifacts)
}

func NormalizeDesiredArtifacts(artifacts []DesiredArtifact) ([]DesiredArtifact, error) {
	if len(artifacts) == 0 {
		return nil, nil
	}

	normalized := make([]DesiredArtifact, 0, len(artifacts))
	seen := map[string]DesiredArtifact{}

	for _, artifact := range artifacts {
		artifact = DesiredArtifact{
			ArtifactURI: strings.TrimSpace(artifact.ArtifactURI),
			Digest:      strings.TrimSpace(artifact.Digest),
			Family:      strings.TrimSpace(artifact.Family),
			SizeBytes:   artifact.SizeBytes,
		}
		switch {
		case artifact.ArtifactURI == "":
			return nil, errors.New("node cache desired artifact URI must not be empty")
		case artifact.Digest == "":
			return nil, errors.New("node cache desired digest must not be empty")
		}

		existing, found := seen[artifact.Digest]
		if !found {
			seen[artifact.Digest] = artifact
			normalized = append(normalized, artifact)
			continue
		}
		merged, err := mergeDesiredArtifact(existing, artifact)
		if err != nil {
			return nil, err
		}
		if merged != existing {
			seen[artifact.Digest] = merged
			replaceDesiredArtifact(normalized, merged)
		}
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Digest < normalized[j].Digest
	})
	return normalized, nil
}

func mergeDesiredArtifact(existing, artifact DesiredArtifact) (DesiredArtifact, error) {
	if existing.ArtifactURI != artifact.ArtifactURI {
		return DesiredArtifact{}, fmt.Errorf("node cache desired digest %q maps to multiple artifact URIs", artifact.Digest)
	}
	if existing.Family != "" && artifact.Family != "" && existing.Family != artifact.Family {
		return DesiredArtifact{}, fmt.Errorf("node cache desired digest %q maps to multiple artifact families", artifact.Digest)
	}
	if existing.SizeBytes > 0 && artifact.SizeBytes > 0 && existing.SizeBytes != artifact.SizeBytes {
		return DesiredArtifact{}, fmt.Errorf("node cache desired digest %q maps to multiple artifact sizes", artifact.Digest)
	}

	merged := existing
	if merged.Family == "" && artifact.Family != "" {
		merged.Family = artifact.Family
	}
	if merged.SizeBytes <= 0 && artifact.SizeBytes > 0 {
		merged.SizeBytes = artifact.SizeBytes
	}
	return merged, nil
}

func replaceDesiredArtifact(artifacts []DesiredArtifact, replacement DesiredArtifact) {
	for index := range artifacts {
		if artifacts[index].Digest == replacement.Digest {
			artifacts[index] = replacement
			return
		}
	}
}

func ProtectedDigests(artifacts []DesiredArtifact) []string {
	normalized, err := NormalizeDesiredArtifacts(artifacts)
	if err != nil {
		return nil
	}
	protected := make([]string, 0, len(normalized))
	for _, artifact := range normalized {
		protected = append(protected, artifact.Digest)
	}
	return protected
}
