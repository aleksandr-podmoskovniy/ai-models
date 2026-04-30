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

const (
	ResolvedDigestAnnotation         = "ai.deckhouse.io/resolved-digest"
	ResolvedArtifactURIAnnotation    = "ai.deckhouse.io/resolved-artifact-uri"
	ResolvedArtifactFamilyAnnotation = "ai.deckhouse.io/resolved-artifact-family"
	ResolvedDeliveryModeAnnotation   = "ai.deckhouse.io/resolved-delivery-mode"
	ResolvedDeliveryReasonAnnotation = "ai.deckhouse.io/resolved-delivery-reason"
	ResolvedModelsAnnotation         = "ai.deckhouse.io/resolved-models"
	ResolvedSignatureAnnotation      = "ai.deckhouse.io/resolved-signature"

	DeliveryAuthKeyEnv = "AI_MODELS_DELIVERY_AUTH_KEY"

	DeliveryModeSharedDirect = "SharedDirect"
	DeliveryModeSharedPVC    = "SharedPVC"

	DeliveryReasonNodeSharedRuntimePlane = "NodeSharedRuntimePlane"
	DeliveryReasonRWXSharedVolume        = "RWXSharedVolume"
)
