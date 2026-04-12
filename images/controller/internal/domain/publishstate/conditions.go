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
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func publishingStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
) modelsv1alpha1.ModelStatus {
	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              modelsv1alpha1.ModelPhasePublishing,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: sourceType,
		},
		Conditions: keepNonPublishConditions(current.Conditions),
	}

	setAcceptedCondition(&status.Conditions, generation)
	return status
}

func pendingUploadStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
) modelsv1alpha1.ModelStatus {
	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              modelsv1alpha1.ModelPhasePending,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: sourceType,
		},
		Conditions: keepNonPublishConditions(current.Conditions),
	}

	setAcceptedCondition(&status.Conditions, generation)
	return status
}

func waitForUploadStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	upload *modelsv1alpha1.ModelUploadStatus,
) modelsv1alpha1.ModelStatus {
	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              modelsv1alpha1.ModelPhaseWaitForUpload,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: sourceType,
		},
		Upload:     upload,
		Conditions: keepNonPublishConditions(current.Conditions),
	}

	setAcceptedCondition(&status.Conditions, generation)
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionUploadReady),
		Status:             metav1.ConditionTrue,
		Reason:             string(modelsv1alpha1.ModelConditionReasonWaitingForUserUpload),
		Message:            "controller prepared an upload session and is waiting for the user upload",
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionReady),
		Status:             metav1.ConditionFalse,
		Reason:             string(modelsv1alpha1.ModelConditionReasonWaitingForUserUpload),
		Message:            "model is waiting for a user upload before publication can continue",
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})

	return status
}

func failedStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	reason modelsv1alpha1.ModelConditionReason,
	message string,
) modelsv1alpha1.ModelStatus {
	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              modelsv1alpha1.ModelPhaseFailed,
		Conditions:         keepNonPublishConditions(current.Conditions),
	}
	if strings.TrimSpace(string(sourceType)) != "" {
		status.Source = &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: sourceType,
		}
	}
	if reason == "" {
		reason = modelsv1alpha1.ModelConditionReasonPublicationFailed
	}

	setAcceptedCondition(&status.Conditions, generation)
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionArtifactPublished),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionReady),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})

	return status
}

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

	setAcceptedCondition(&status.Conditions, generation)
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionArtifactPublished),
		Status:             metav1.ConditionTrue,
		Reason:             string(modelsv1alpha1.ModelConditionReasonPublicationSucceeded),
		Message:            "controller published the model artifact successfully",
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionMetadataReady),
		Status:             metav1.ConditionTrue,
		Reason:             string(modelsv1alpha1.ModelConditionReasonMetadataInspectionSucceeded),
		Message:            "controller resolved the model technical profile successfully",
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionValidated),
		Status:             conditionStatus(validation.Valid),
		Reason:             string(validation.Reason),
		Message:            validation.Message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionReady),
		Status:             conditionStatus(validation.Valid),
		Reason:             string(validation.Reason),
		Message:            readyMessage(validation),
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})

	return status
}

func conditionStatus(ok bool) metav1.ConditionStatus {
	if ok {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func readyMessage(validation policyValidationResult) string {
	if validation.Valid {
		return "model is ready for platform consumption"
	}
	return validation.Message
}

func keepNonPublishConditions(conditions []metav1.Condition) []metav1.Condition {
	result := make([]metav1.Condition, 0, len(conditions))
	for _, condition := range conditions {
		switch modelsv1alpha1.ModelConditionType(condition.Type) {
		case modelsv1alpha1.ModelConditionAccepted,
			modelsv1alpha1.ModelConditionUploadReady,
			modelsv1alpha1.ModelConditionArtifactPublished,
			modelsv1alpha1.ModelConditionMetadataReady,
			modelsv1alpha1.ModelConditionValidated,
			modelsv1alpha1.ModelConditionReady:
			continue
		default:
			result = append(result, condition)
		}
	}

	return result
}

func setAcceptedCondition(conditions *[]metav1.Condition, generation int64) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(modelsv1alpha1.ModelConditionReasonSpecAccepted),
		Message:            "model spec was accepted by the controller",
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}
