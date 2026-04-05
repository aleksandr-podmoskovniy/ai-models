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

const (
	ClusterModelKind     = "ClusterModel"
	ClusterModelResource = "clustermodels"
)

// ClusterModel is the cluster-scoped public catalog object for curated shared
// published models. Unlike Model, access policy for ClusterModel is expected to
// be explicit rather than inferred from the object's namespace.
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:validation:XValidation:rule="has(self.spec.access)",message="spec.access is required for ClusterModel"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.access) || !has(self.spec.access.serviceAccounts) || self.spec.access.serviceAccounts.all(sa, has(sa.namespace) && size(sa.namespace) > 0)",message="spec.access.serviceAccounts.namespace is required for ClusterModel"
// +kubebuilder:validation:XValidation:rule="self.spec.source.type != 'HuggingFace' || !has(self.spec.source.huggingFace) || !has(self.spec.source.huggingFace.authSecretRef) || (has(self.spec.source.huggingFace.authSecretRef.namespace) && size(self.spec.source.huggingFace.authSecretRef.namespace) > 0)",message="spec.source.huggingFace.authSecretRef.namespace is required for ClusterModel"
// +kubebuilder:validation:XValidation:rule="self.spec.source.type != 'HTTP' || !has(self.spec.source.http) || !has(self.spec.source.http.authSecretRef) || (has(self.spec.source.http.authSecretRef.namespace) && size(self.spec.source.http.authSecretRef.namespace) > 0)",message="spec.source.http.authSecretRef.namespace is required for ClusterModel"
// +kubebuilder:metadata:labels={heritage=deckhouse,module=ai-models}
// +kubebuilder:resource:categories={ai-models},scope=Cluster,singular=clustermodel
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.type`
// +kubebuilder:printcolumn:name="ArtifactURI",type=string,JSONPath=`.status.artifact.uri`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ClusterModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelSpec   `json:"spec"`
	Status ModelStatus `json:"status,omitempty"`
}

// ClusterModelList contains a list of ClusterModel objects.
// +kubebuilder:object:root=true
type ClusterModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ClusterModel `json:"items"`
}
