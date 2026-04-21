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
	ModelConditionArtifactResolved ModelConditionType = "ArtifactResolved"
	ModelConditionMetadataResolved ModelConditionType = "MetadataResolved"
	ModelConditionValidated        ModelConditionType = "Validated"
	ModelConditionReady            ModelConditionType = "Ready"
)

type ModelConditionReason string

const (
	ModelConditionReasonPending                   ModelConditionReason = "Pending"
	ModelConditionReasonWaitingForUserUpload      ModelConditionReason = "WaitingForUserUpload"
	ModelConditionReasonPublicationStarted        ModelConditionReason = "PublicationStarted"
	ModelConditionReasonPublicationUploading      ModelConditionReason = "PublicationUploading"
	ModelConditionReasonPublicationResumed        ModelConditionReason = "PublicationResumed"
	ModelConditionReasonPublicationSealing        ModelConditionReason = "PublicationSealing"
	ModelConditionReasonPublicationCommitted      ModelConditionReason = "PublicationCommitted"
	ModelConditionReasonArtifactPublished         ModelConditionReason = "ArtifactPublished"
	ModelConditionReasonPublicationFailed         ModelConditionReason = "PublicationFailed"
	ModelConditionReasonUnsupportedSource         ModelConditionReason = "UnsupportedSource"
	ModelConditionReasonModelMetadataCalculated   ModelConditionReason = "ModelMetadataCalculated"
	ModelConditionReasonValidationSucceeded       ModelConditionReason = "ValidationSucceeded"
	ModelConditionReasonModelTypeMismatch         ModelConditionReason = "ModelTypeMismatch"
	ModelConditionReasonEndpointTypeNotSupported  ModelConditionReason = "EndpointTypeNotSupported"
	ModelConditionReasonRuntimeNotSupported       ModelConditionReason = "RuntimeNotSupported"
	ModelConditionReasonAcceleratorPolicyConflict ModelConditionReason = "AcceleratorPolicyConflict"
	ModelConditionReasonOptimizationNotSupported  ModelConditionReason = "OptimizationNotSupported"
	ModelConditionReasonReady                     ModelConditionReason = "Ready"
	ModelConditionReasonFailed                    ModelConditionReason = "Failed"
)
