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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func readyStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	snapshot publicationdata.Snapshot,
) modelsv1alpha1.ModelStatus {
	status := modelsv1alpha1.ModelStatus{
		ObservedGeneration: generation,
		Phase:              modelsv1alpha1.ModelPhaseReady,
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
			SupportedEndpointTypes: publicEndpointTypes(snapshot.Resolved),
		},
		Conditions: keepNonPublishConditions(current.Conditions),
	}

	if snapshot.Artifact.SizeBytes > 0 {
		sizeBytes := snapshot.Artifact.SizeBytes
		status.Artifact.SizeBytes = &sizeBytes
	}
	if snapshot.Resolved.TaskConfidence.ReliablePublicFact() {
		status.Resolved.Task = snapshot.Resolved.Task
	}
	if format, ok := knownPublicFormat(modelsv1alpha1.ModelInputFormat(snapshot.Resolved.Format)); ok {
		status.Resolved.Format = format
	}
	if snapshot.Resolved.FamilyConfidence.ReliablePublicFact() {
		status.Resolved.Family = snapshot.Resolved.Family
	}
	if snapshot.Resolved.ArchitectureConfidence.ReliablePublicFact() {
		status.Resolved.Architecture = snapshot.Resolved.Architecture
	}
	if snapshot.Resolved.ContextWindowTokens > 0 && snapshot.Resolved.ContextWindowTokensConfidence.ReliablePublicFact() {
		contextWindow := snapshot.Resolved.ContextWindowTokens
		status.Resolved.ContextWindowTokens = &contextWindow
	}
	if snapshot.Resolved.ParameterCount > 0 && snapshot.Resolved.ParameterCountConfidence.ReliablePublicFact() {
		parameterCount := snapshot.Resolved.ParameterCount
		status.Resolved.ParameterCount = &parameterCount
	}
	if snapshot.Resolved.Quantization != "" && snapshot.Resolved.QuantizationConfidence.ReliablePublicFact() {
		quantization := snapshot.Resolved.Quantization
		status.Resolved.Quantization = &quantization
	}

	setArtifactResolvedCondition(
		&status.Conditions,
		generation,
		metav1.ConditionTrue,
		modelsv1alpha1.ModelConditionReasonArtifactPublished,
		"controller published the model artifact successfully",
	)
	metadataReason, metadataMessage := metadataCondition(snapshot.Resolved)
	setMetadataResolvedCondition(
		&status.Conditions,
		generation,
		metav1.ConditionTrue,
		metadataReason,
		metadataMessage,
	)
	setReadyCondition(
		&status.Conditions,
		generation,
		metav1.ConditionTrue,
		modelsv1alpha1.ModelConditionReasonReady,
		"model is ready for platform consumption",
	)

	return status
}

func publicEndpointTypes(resolved publicationdata.ResolvedProfile) []modelsv1alpha1.ModelEndpointType {
	if !resolved.TaskConfidence.ReliablePublicFact() {
		return nil
	}
	result := make([]modelsv1alpha1.ModelEndpointType, 0, len(resolved.SupportedEndpointTypes))
	for _, endpoint := range resolved.SupportedEndpointTypes {
		endpointType, ok := knownPublicEndpointType(modelsv1alpha1.ModelEndpointType(endpoint))
		if !ok {
			continue
		}
		result = append(result, endpointType)
	}
	return result
}

func knownPublicFormat(format modelsv1alpha1.ModelInputFormat) (modelsv1alpha1.ModelInputFormat, bool) {
	switch format {
	case modelsv1alpha1.ModelInputFormatSafetensors,
		modelsv1alpha1.ModelInputFormatGGUF:
		return format, true
	default:
		return "", false
	}
}

func knownPublicEndpointType(endpoint modelsv1alpha1.ModelEndpointType) (modelsv1alpha1.ModelEndpointType, bool) {
	switch endpoint {
	case modelsv1alpha1.ModelEndpointTypeChat,
		modelsv1alpha1.ModelEndpointTypeTextGeneration,
		modelsv1alpha1.ModelEndpointTypeEmbeddings,
		modelsv1alpha1.ModelEndpointTypeRerank,
		modelsv1alpha1.ModelEndpointTypeSpeechToText,
		modelsv1alpha1.ModelEndpointTypeTranslation:
		return endpoint, true
	default:
		return "", false
	}
}

func metadataCondition(resolved publicationdata.ResolvedProfile) (modelsv1alpha1.ModelConditionReason, string) {
	if resolved.HasPartialConfidence() {
		return modelsv1alpha1.ModelConditionReasonModelMetadataPartial,
			"controller resolved partial model metadata and omitted low-confidence fields from public status"
	}
	return modelsv1alpha1.ModelConditionReasonModelMetadataCalculated,
		"controller resolved the model technical profile successfully"
}
