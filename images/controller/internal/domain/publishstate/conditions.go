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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func publishingStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	reason modelsv1alpha1.ModelConditionReason,
	message string,
	progress string,
) modelsv1alpha1.ModelStatus {
	if reason == "" {
		reason = modelsv1alpha1.ModelConditionReasonPending
	}
	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              modelsv1alpha1.ModelPhasePublishing,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: sourceType,
		},
		Conditions: keepNonPublishConditions(current.Conditions),
	}
	if strings.TrimSpace(progress) != "" {
		status.Progress = strings.TrimSpace(progress)
	}

	setArtifactResolvedCondition(
		&status.Conditions,
		generation,
		metav1.ConditionFalse,
		reason,
		defaultRunningMessage(message, "controller is publishing the model artifact"),
	)
	setReadyCondition(
		&status.Conditions,
		generation,
		metav1.ConditionFalse,
		reason,
		defaultRunningMessage(message, "model publication is in progress"),
	)
	return status
}

func pendingUploadStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	progress string,
	message string,
) modelsv1alpha1.ModelStatus {
	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              modelsv1alpha1.ModelPhasePending,
		Progress:           defaultUploadProgress(progress),
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: sourceType,
		},
		Conditions: keepNonPublishConditions(current.Conditions),
	}

	setArtifactResolvedCondition(
		&status.Conditions,
		generation,
		metav1.ConditionFalse,
		modelsv1alpha1.ModelConditionReasonPending,
		defaultRunningMessage(message, "controller is preparing the upload publication flow"),
	)
	setReadyCondition(
		&status.Conditions,
		generation,
		metav1.ConditionFalse,
		modelsv1alpha1.ModelConditionReasonPending,
		defaultRunningMessage(message, "model is not ready until the upload publication flow starts"),
	)
	return status
}

func waitForUploadStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	upload *modelsv1alpha1.ModelUploadStatus,
	progress string,
	message string,
) modelsv1alpha1.ModelStatus {
	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              modelsv1alpha1.ModelPhaseWaitForUpload,
		Progress:           defaultUploadProgress(progress),
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: sourceType,
		},
		Upload:     upload,
		Conditions: keepNonPublishConditions(current.Conditions),
	}

	setArtifactResolvedCondition(
		&status.Conditions,
		generation,
		metav1.ConditionFalse,
		modelsv1alpha1.ModelConditionReasonWaitingForUserUpload,
		defaultRunningMessage(message, "controller prepared an upload session and is waiting for the user upload"),
	)
	setReadyCondition(
		&status.Conditions,
		generation,
		metav1.ConditionFalse,
		modelsv1alpha1.ModelConditionReasonPending,
		defaultRunningMessage(message, "model is waiting for a user upload before publication can continue"),
	)

	return status
}

func defaultRunningMessage(message string, fallback string) string {
	message = strings.TrimSpace(message)
	if message != "" {
		return message
	}
	return fallback
}

func defaultUploadProgress(progress string) string {
	progress = strings.TrimSpace(progress)
	if progress != "" {
		return progress
	}
	return "0%"
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

	setArtifactResolvedCondition(&status.Conditions, generation, metav1.ConditionFalse, reason, message)
	setReadyCondition(
		&status.Conditions,
		generation,
		metav1.ConditionFalse,
		modelsv1alpha1.ModelConditionReasonFailed,
		message,
	)

	return status
}
