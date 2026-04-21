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
// published models.
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:validation:XValidation:rule="!has(self.spec.source.authSecretRef) || (has(self.spec.source.authSecretRef.namespace) && size(self.spec.source.authSecretRef.namespace) > 0)",message="spec.source.authSecretRef.namespace is required for ClusterModel"
// +kubebuilder:metadata:labels={heritage=deckhouse,module=ai-models}
// +kubebuilder:resource:categories={ai-models},scope=Cluster,singular=clustermodel
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Progress",type=string,JSONPath=`.status.progress`,priority=1
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.status.source.resolvedType`
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
