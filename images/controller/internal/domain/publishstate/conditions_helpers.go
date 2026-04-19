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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		case modelsv1alpha1.ModelConditionArtifactResolved,
			modelsv1alpha1.ModelConditionMetadataResolved,
			modelsv1alpha1.ModelConditionValidated,
			modelsv1alpha1.ModelConditionReady:
			continue
		default:
			result = append(result, condition)
		}
	}

	return result
}

func setArtifactResolvedCondition(
	conditions *[]metav1.Condition,
	generation int64,
	status metav1.ConditionStatus,
	reason modelsv1alpha1.ModelConditionReason,
	message string,
) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionArtifactResolved),
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}

func setMetadataResolvedCondition(
	conditions *[]metav1.Condition,
	generation int64,
	status metav1.ConditionStatus,
	reason modelsv1alpha1.ModelConditionReason,
	message string,
) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionMetadataResolved),
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}

func setReadyCondition(
	conditions *[]metav1.Condition,
	generation int64,
	status metav1.ConditionStatus,
	reason modelsv1alpha1.ModelConditionReason,
	message string,
) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionReady),
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}

func readyReason(ok bool) modelsv1alpha1.ModelConditionReason {
	if ok {
		return modelsv1alpha1.ModelConditionReasonReady
	}
	return modelsv1alpha1.ModelConditionReasonFailed
}
