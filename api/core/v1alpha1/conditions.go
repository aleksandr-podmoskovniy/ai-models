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

package v1alpha1

type ModelConditionType string

const (
	ModelConditionAccepted          ModelConditionType = "Accepted"
	ModelConditionUploadReady       ModelConditionType = "UploadReady"
	ModelConditionArtifactPublished ModelConditionType = "ArtifactPublished"
	ModelConditionMetadataReady     ModelConditionType = "MetadataReady"
	ModelConditionValidated         ModelConditionType = "Validated"
	ModelConditionCleanupCompleted  ModelConditionType = "CleanupCompleted"
	ModelConditionReady             ModelConditionType = "Ready"
)

type ModelConditionReason string

const (
	ModelConditionReasonSpecAccepted                ModelConditionReason = "SpecAccepted"
	ModelConditionReasonWaitingForUserUpload        ModelConditionReason = "WaitingForUserUpload"
	ModelConditionReasonUploadExpired               ModelConditionReason = "UploadExpired"
	ModelConditionReasonPublicationSucceeded        ModelConditionReason = "PublicationSucceeded"
	ModelConditionReasonPublicationFailed           ModelConditionReason = "PublicationFailed"
	ModelConditionReasonUnsupportedSource           ModelConditionReason = "UnsupportedSource"
	ModelConditionReasonMetadataInspectionSucceeded ModelConditionReason = "MetadataInspectionSucceeded"
	ModelConditionReasonMetadataInspectionFailed    ModelConditionReason = "MetadataInspectionFailed"
	ModelConditionReasonValidationSucceeded         ModelConditionReason = "ValidationSucceeded"
	ModelConditionReasonValidationFailed            ModelConditionReason = "ValidationFailed"
	ModelConditionReasonModelTypeMismatch           ModelConditionReason = "ModelTypeMismatch"
	ModelConditionReasonEndpointTypeNotSupported    ModelConditionReason = "EndpointTypeNotSupported"
	ModelConditionReasonRuntimeNotSupported         ModelConditionReason = "RuntimeNotSupported"
	ModelConditionReasonAcceleratorPolicyConflict   ModelConditionReason = "AcceleratorPolicyConflict"
	ModelConditionReasonOptimizationNotSupported    ModelConditionReason = "OptimizationNotSupported"
	ModelConditionReasonCleanupPending              ModelConditionReason = "CleanupPending"
	ModelConditionReasonCleanupBlocked              ModelConditionReason = "CleanupBlocked"
	ModelConditionReasonCleanupFailed               ModelConditionReason = "CleanupFailed"
)
