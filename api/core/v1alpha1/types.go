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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ModelSpec is the shared desired state for both Model and ClusterModel.
// +kubebuilder:validation:XValidation:rule="self.source == oldSelf.source",message="spec.source is immutable"
type ModelSpec struct {
	Source      ModelSourceSpec `json:"source"`
}

// ModelStatus is the shared observed state for both Model and ClusterModel.
// It intentionally exposes only public-contract-facing state.
type ModelStatus struct {
	ObservedGeneration int64                 `json:"observedGeneration,omitempty"`
	Phase              ModelPhase            `json:"phase,omitempty"`
	Source             *ResolvedSourceStatus `json:"source,omitempty"`
	Upload             *ModelUploadStatus    `json:"upload,omitempty"`
	Artifact           *ModelArtifactStatus  `json:"artifact,omitempty"`
	Resolved           *ModelResolvedStatus  `json:"resolved,omitempty"`
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:validation:XValidation:rule="((has(self.url) && size(self.url) > 0 ? 1 : 0) + (has(self.upload) ? 1 : 0)) == 1",message="exactly one of source.url or source.upload must be specified"
// +kubebuilder:validation:XValidation:rule="has(self.upload) ? !has(self.authSecretRef) : true",message="source.authSecretRef is only allowed for source.url"
type ModelSourceSpec struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^https:\/\/((www\.)?huggingface\.co|hf\.co)\/.+$`
	URL string `json:"url,omitempty"`
	// For namespaced Model, empty Namespace resolves to the object's namespace.
	AuthSecretRef *SecretReference   `json:"authSecretRef,omitempty"`
	Upload        *UploadModelSource `json:"upload,omitempty"`
}

type UploadModelSource struct{}

// SecretReference identifies a Secret used by a source integration.
// For namespaced Model, empty Namespace resolves to the object's namespace.
type SecretReference struct {
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type ResolvedSourceStatus struct {
	ResolvedType     ModelSourceType `json:"resolvedType,omitempty"`
	ResolvedRevision string          `json:"resolvedRevision,omitempty"`
}

type ModelUploadStatus struct {
	ExpiresAt    *metav1.Time `json:"expiresAt,omitempty"`
	Repository   string       `json:"repository,omitempty"`
	ExternalURL  string       `json:"externalURL,omitempty"`
	InClusterURL string       `json:"inClusterURL,omitempty"`
}

type ModelArtifactStatus struct {
	Kind      ModelArtifactLocationKind `json:"kind,omitempty"`
	URI       string                    `json:"uri,omitempty"`
	Digest    string                    `json:"digest,omitempty"`
	MediaType string                    `json:"mediaType,omitempty"`
	// +kubebuilder:validation:Minimum=1
	SizeBytes *int64 `json:"sizeBytes,omitempty"`
}

type ModelResolvedStatus struct {
	Task         string `json:"task,omitempty"`
	Framework    string `json:"framework,omitempty"`
	Family       string `json:"family,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Format       string `json:"format,omitempty"`
	// +kubebuilder:validation:Minimum=1
	ParameterCount *int64  `json:"parameterCount,omitempty"`
	Quantization   *string `json:"quantization,omitempty"`
	// +kubebuilder:validation:Minimum=1
	ContextWindowTokens *int64 `json:"contextWindowTokens,omitempty"`
	// +listType=set
	SupportedEndpointTypes []string `json:"supportedEndpointTypes,omitempty"`
	// +listType=set
	CompatibleRuntimes []string `json:"compatibleRuntimes,omitempty"`
	// +listType=set
	CompatibleAcceleratorVendors []string `json:"compatibleAcceleratorVendors,omitempty"`
	// +listType=set
	CompatiblePrecisions []string                  `json:"compatiblePrecisions,omitempty"`
	MinimumLaunch        *ModelMinimumLaunchStatus `json:"minimumLaunch,omitempty"`
}

type ModelMinimumLaunchStatus struct {
	PlacementType string `json:"placementType,omitempty"`
	// +kubebuilder:validation:Minimum=1
	AcceleratorCount *int64 `json:"acceleratorCount,omitempty"`
	// +kubebuilder:validation:Minimum=1
	AcceleratorMemoryGiB *int64 `json:"acceleratorMemoryGiB,omitempty"`
	SharingMode          string `json:"sharingMode,omitempty"`
}

// +kubebuilder:validation:Enum=HuggingFace;Upload
type ModelSourceType string

const (
	ModelSourceTypeHuggingFace ModelSourceType = "HuggingFace"
	ModelSourceTypeUpload      ModelSourceType = "Upload"
)

// +kubebuilder:validation:Enum=Safetensors;GGUF
type ModelInputFormat string

const (
	ModelInputFormatSafetensors ModelInputFormat = "Safetensors"
	ModelInputFormatGGUF        ModelInputFormat = "GGUF"
)

// +kubebuilder:validation:Enum=VLLM;Ollama;TGI;Custom
type ModelRuntimeEngine string

const (
	ModelRuntimeEngineVLLM   ModelRuntimeEngine = "VLLM"
	ModelRuntimeEngineOllama ModelRuntimeEngine = "Ollama"
	ModelRuntimeEngineTGI    ModelRuntimeEngine = "TGI"
	ModelRuntimeEngineCustom ModelRuntimeEngine = "Custom"
)

// +kubebuilder:validation:Enum=OCI
type ModelArtifactLocationKind string

const (
	ModelArtifactLocationKindOCI ModelArtifactLocationKind = "OCI"
)

// +kubebuilder:validation:Enum=Pending;WaitForUpload;Publishing;Ready;Failed;Deleting
type ModelPhase string

const (
	ModelPhasePending       ModelPhase = "Pending"
	ModelPhaseWaitForUpload ModelPhase = "WaitForUpload"
	ModelPhasePublishing    ModelPhase = "Publishing"
	ModelPhaseReady         ModelPhase = "Ready"
	ModelPhaseFailed        ModelPhase = "Failed"
	ModelPhaseDeleting      ModelPhase = "Deleting"
)
