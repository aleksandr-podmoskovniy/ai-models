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

// +kubebuilder:validation:Enum=LLM;Embeddings;Reranker;SpeechToText;Translation
type ModelType string

const (
	ModelTypeLLM          ModelType = "LLM"
	ModelTypeEmbeddings   ModelType = "Embeddings"
	ModelTypeReranker     ModelType = "Reranker"
	ModelTypeSpeechToText ModelType = "SpeechToText"
	ModelTypeTranslation  ModelType = "Translation"
)

// +kubebuilder:validation:Enum=Chat;TextGeneration;Embeddings;Rerank;SpeechToText;Translation
type ModelEndpointType string

const (
	ModelEndpointTypeChat           ModelEndpointType = "Chat"
	ModelEndpointTypeTextGeneration ModelEndpointType = "TextGeneration"
	ModelEndpointTypeEmbeddings     ModelEndpointType = "Embeddings"
	ModelEndpointTypeRerank         ModelEndpointType = "Rerank"
	ModelEndpointTypeSpeechToText   ModelEndpointType = "SpeechToText"
	ModelEndpointTypeTranslation    ModelEndpointType = "Translation"
)

// +kubebuilder:validation:Enum=NVIDIA;AMD;Intel
type ModelAcceleratorVendor string

const (
	ModelAcceleratorVendorNVIDIA ModelAcceleratorVendor = "NVIDIA"
	ModelAcceleratorVendorAMD    ModelAcceleratorVendor = "AMD"
	ModelAcceleratorVendorIntel  ModelAcceleratorVendor = "Intel"
)

// +kubebuilder:validation:Enum=FP32;FP16;BF16;FP8;INT8;INT4
type ModelPrecision string

const (
	ModelPrecisionFP32 ModelPrecision = "FP32"
	ModelPrecisionFP16 ModelPrecision = "FP16"
	ModelPrecisionBF16 ModelPrecision = "BF16"
	ModelPrecisionFP8  ModelPrecision = "FP8"
	ModelPrecisionINT8 ModelPrecision = "INT8"
	ModelPrecisionINT4 ModelPrecision = "INT4"
)

// +kubebuilder:validation:Enum=Model;ClusterModel
type ModelReferenceKind string

const (
	ModelReferenceKindModel        ModelReferenceKind = "Model"
	ModelReferenceKindClusterModel ModelReferenceKind = "ClusterModel"
)

type ModelUsagePolicy struct {
	// +listType=set
	AllowedEndpointTypes []ModelEndpointType `json:"allowedEndpointTypes,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!has(self.preferredRuntime) || !(size(self.allowedRuntimes) > 0) || self.preferredRuntime in self.allowedRuntimes",message="launchPolicy.preferredRuntime must be included in launchPolicy.allowedRuntimes"
type ModelLaunchPolicy struct {
	// +listType=set
	AllowedRuntimes  []ModelRuntimeEngine `json:"allowedRuntimes,omitempty"`
	PreferredRuntime ModelRuntimeEngine   `json:"preferredRuntime,omitempty"`
	// +listType=set
	AllowedAcceleratorVendors []ModelAcceleratorVendor `json:"allowedAcceleratorVendors,omitempty"`
	// +listType=set
	AllowedPrecisions []ModelPrecision `json:"allowedPrecisions,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="size(self.kind) > 0 && size(self.name) > 0",message="optimization.speculativeDecoding.draftModelRefs items must contain kind and name"
// +kubebuilder:validation:XValidation:rule="self.kind != 'ClusterModel' ? true : size(self.namespace) == 0",message="namespace must be empty for ClusterModel references"
type ModelReference struct {
	Kind ModelReferenceKind `json:"kind"`
	// For `Model`, empty namespace resolves to the owner's namespace.
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

type ModelSpeculativeDecodingPolicy struct {
	// +listType=atomic
	DraftModelRefs []ModelReference `json:"draftModelRefs,omitempty"`
}

type ModelOptimizationPolicy struct {
	SpeculativeDecoding *ModelSpeculativeDecodingPolicy `json:"speculativeDecoding,omitempty"`
}
