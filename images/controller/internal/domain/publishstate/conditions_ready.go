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

package publishstate

import (
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func readyStatus(
	current modelsv1alpha1.ModelStatus,
	spec modelsv1alpha1.ModelSpec,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	snapshot publicationdata.Snapshot,
) modelsv1alpha1.ModelStatus {
	validation := validatePublishedModel(spec, snapshot)
	phase := modelsv1alpha1.ModelPhaseReady
	if !validation.Valid {
		phase = modelsv1alpha1.ModelPhaseFailed
	}

	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              phase,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType:     sourceType,
			ResolvedRevision: snapshot.Source.ResolvedRevision,
		},
		Artifact: &modelsv1alpha1.ModelArtifactStatus{
			Kind:      snapshot.Artifact.Kind,
			URI:       snapshot.Artifact.URI,
			Digest:    snapshot.Artifact.Digest,
			MediaType: snapshot.Artifact.MediaType,
		},
		Resolved: &modelsv1alpha1.ModelResolvedStatus{
			Task:                         snapshot.Resolved.Task,
			Framework:                    snapshot.Resolved.Framework,
			Family:                       snapshot.Resolved.Family,
			Architecture:                 snapshot.Resolved.Architecture,
			Format:                       snapshot.Resolved.Format,
			SupportedEndpointTypes:       append([]string(nil), snapshot.Resolved.SupportedEndpointTypes...),
			CompatibleRuntimes:           append([]string(nil), snapshot.Resolved.CompatibleRuntimes...),
			CompatibleAcceleratorVendors: append([]string(nil), snapshot.Resolved.CompatibleAcceleratorVendors...),
			CompatiblePrecisions:         append([]string(nil), snapshot.Resolved.CompatiblePrecisions...),
		},
		Conditions: keepNonPublishConditions(current.Conditions),
	}

	if snapshot.Artifact.SizeBytes > 0 {
		sizeBytes := snapshot.Artifact.SizeBytes
		status.Artifact.SizeBytes = &sizeBytes
	}
	if snapshot.Resolved.ContextWindowTokens > 0 {
		contextWindow := snapshot.Resolved.ContextWindowTokens
		status.Resolved.ContextWindowTokens = &contextWindow
	}
	if snapshot.Resolved.ParameterCount > 0 {
		parameterCount := snapshot.Resolved.ParameterCount
		status.Resolved.ParameterCount = &parameterCount
	}
	if snapshot.Resolved.Quantization != "" {
		quantization := snapshot.Resolved.Quantization
		status.Resolved.Quantization = &quantization
	}
	if snapshot.Resolved.MinimumLaunch.PlacementType != "" {
		minimumLaunch := &modelsv1alpha1.ModelMinimumLaunchStatus{
			PlacementType: snapshot.Resolved.MinimumLaunch.PlacementType,
			SharingMode:   snapshot.Resolved.MinimumLaunch.SharingMode,
		}
		if snapshot.Resolved.MinimumLaunch.AcceleratorCount > 0 {
			acceleratorCount := snapshot.Resolved.MinimumLaunch.AcceleratorCount
			minimumLaunch.AcceleratorCount = &acceleratorCount
		}
		if snapshot.Resolved.MinimumLaunch.AcceleratorMemoryGiB > 0 {
			acceleratorMemory := snapshot.Resolved.MinimumLaunch.AcceleratorMemoryGiB
			minimumLaunch.AcceleratorMemoryGiB = &acceleratorMemory
		}
		status.Resolved.MinimumLaunch = minimumLaunch
	}

	setArtifactResolvedCondition(
		&status.Conditions,
		generation,
		metav1.ConditionTrue,
		modelsv1alpha1.ModelConditionReasonArtifactPublished,
		"controller published the model artifact successfully",
	)
	setMetadataResolvedCondition(
		&status.Conditions,
		generation,
		metav1.ConditionTrue,
		modelsv1alpha1.ModelConditionReasonModelMetadataCalculated,
		"controller resolved the model technical profile successfully",
	)
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionValidated),
		Status:             conditionStatus(validation.Valid),
		Reason:             string(validation.Reason),
		Message:            validation.Message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
	setReadyCondition(
		&status.Conditions,
		generation,
		conditionStatus(validation.Valid),
		readyReason(validation.Valid),
		readyMessage(validation),
	)

	return status
}
