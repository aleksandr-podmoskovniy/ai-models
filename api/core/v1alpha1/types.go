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
// +kubebuilder:validation:XValidation:rule="self.package == oldSelf.package",message="spec.package is immutable"
// +kubebuilder:validation:XValidation:rule="self.publish == oldSelf.publish",message="spec.publish is immutable"
// +kubebuilder:validation:XValidation:rule="self.runtimeHints == oldSelf.runtimeHints",message="spec.runtimeHints is immutable"
type ModelSpec struct {
	DisplayName string          `json:"displayName,omitempty"`
	Description string          `json:"description,omitempty"`
	Source      ModelSourceSpec `json:"source"`
	// +kubebuilder:default={type:ModelPack,layout:HuggingFaceCheckpoint}
	Package      *ModelPackageSpec  `json:"package,omitempty"`
	Publish      *ModelPublishSpec  `json:"publish,omitempty"`
	RuntimeHints *ModelRuntimeHints `json:"runtimeHints,omitempty"`
	Access       *ModelAccessPolicy `json:"access,omitempty"`
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

// +kubebuilder:validation:XValidation:rule="self.type == 'HuggingFace' ? has(self.huggingFace) && !has(self.upload) && !has(self.http) : true",message="huggingFace is required when type is HuggingFace and other source variants must be empty"
// +kubebuilder:validation:XValidation:rule="self.type == 'Upload' ? has(self.upload) && !has(self.huggingFace) && !has(self.http) : true",message="upload is required when type is Upload and other source variants must be empty"
// +kubebuilder:validation:XValidation:rule="self.type == 'HTTP' ? has(self.http) && !has(self.huggingFace) && !has(self.upload) : true",message="http is required when type is HTTP and other source variants must be empty"
type ModelSourceSpec struct {
	Type        ModelSourceType         `json:"type"`
	HuggingFace *HuggingFaceModelSource `json:"huggingFace,omitempty"`
	Upload      *UploadModelSource      `json:"upload,omitempty"`
	HTTP        *HTTPModelSource        `json:"http,omitempty"`
}

type HuggingFaceModelSource struct {
	// +kubebuilder:validation:MinLength=1
	RepoID        string           `json:"repoID"`
	Revision      string           `json:"revision,omitempty"`
	AuthSecretRef *SecretReference `json:"authSecretRef,omitempty"`
	AllowGated    bool             `json:"allowGated,omitempty"`
}

type UploadModelSource struct {
	// +kubebuilder:default=HuggingFaceDirectory
	ExpectedFormat ModelUploadFormat `json:"expectedFormat,omitempty"`
	// +kubebuilder:validation:Minimum=1
	ExpectedSizeBytes *int64 `json:"expectedSizeBytes,omitempty"`
}

type HTTPModelSource struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^http[s]?:\/\/(?:[a-zA-Z]|[0-9]|[$-_@.&+]|[!*\(\),]|(?:%[0-9a-fA-F][0-9a-fA-F]))+$`
	URL string `json:"url"`
	// For namespaced Model, empty Namespace resolves to the object's namespace.
	AuthSecretRef *SecretReference `json:"authSecretRef,omitempty"`
	CABundle      []byte           `json:"caBundle,omitempty"`
}

type ModelPackageSpec struct {
	// +kubebuilder:default=ModelPack
	Type ModelPackageType `json:"type,omitempty"`
	// +kubebuilder:default=HuggingFaceCheckpoint
	Layout ModelPackageLayout `json:"layout,omitempty"`
}

type ModelPublishSpec struct {
	RepositoryClass string `json:"repositoryClass,omitempty"`
}

type ModelRuntimeHints struct {
	Task string `json:"task,omitempty"`
	// +listType=set
	Engines []ModelRuntimeEngine `json:"engines,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.namespaces) && size(self.namespaces) > 0) || (has(self.serviceAccounts) && size(self.serviceAccounts) > 0) || (has(self.groups) && size(self.groups) > 0)",message="at least one access subject must be specified"
type ModelAccessPolicy struct {
	// +listType=set
	Namespaces      []string                  `json:"namespaces,omitempty"`
	ServiceAccounts []ServiceAccountReference `json:"serviceAccounts,omitempty"`
	// +listType=set
	Groups []string `json:"groups,omitempty"`
}

// ServiceAccountReference identifies a pull consumer.
// For namespaced Model, empty Namespace resolves to the object's namespace.
// For ClusterModel, Namespace should be explicit.
type ServiceAccountReference struct {
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

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
	ExpiresAt  *metav1.Time `json:"expiresAt,omitempty"`
	Repository string       `json:"repository,omitempty"`
	Command    string       `json:"command,omitempty"`
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
	License      string `json:"license,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Format       string `json:"format,omitempty"`
	// +kubebuilder:validation:Minimum=1
	ParameterCount *int64  `json:"parameterCount,omitempty"`
	Quantization   *string `json:"quantization,omitempty"`
	// +kubebuilder:validation:Minimum=1
	ContextWindowTokens *int64 `json:"contextWindowTokens,omitempty"`
	SourceRepoID        string `json:"sourceRepoID,omitempty"`
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

// +kubebuilder:validation:Enum=HuggingFace;Upload;HTTP
type ModelSourceType string

const (
	ModelSourceTypeHuggingFace ModelSourceType = "HuggingFace"
	ModelSourceTypeUpload      ModelSourceType = "Upload"
	ModelSourceTypeHTTP        ModelSourceType = "HTTP"
)

// +kubebuilder:validation:Enum=ModelKit;HuggingFaceDirectory
type ModelUploadFormat string

const (
	ModelUploadFormatModelKit             ModelUploadFormat = "ModelKit"
	ModelUploadFormatHuggingFaceDirectory ModelUploadFormat = "HuggingFaceDirectory"
)

// +kubebuilder:validation:Enum=ModelPack
type ModelPackageType string

const (
	ModelPackageTypeModelPack ModelPackageType = "ModelPack"
)

// +kubebuilder:validation:Enum=HuggingFaceCheckpoint
type ModelPackageLayout string

const (
	ModelPackageLayoutHuggingFaceCheckpoint ModelPackageLayout = "HuggingFaceCheckpoint"
)

// +kubebuilder:validation:Enum=KServe;KubeRay
type ModelRuntimeEngine string

const (
	ModelRuntimeEngineKServe  ModelRuntimeEngine = "KServe"
	ModelRuntimeEngineKubeRay ModelRuntimeEngine = "KubeRay"
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
