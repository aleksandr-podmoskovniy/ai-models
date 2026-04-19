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

package nodecachesubstrate

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ManagedLabelKey   = "ai.deckhouse.io/node-cache-substrate"
	ManagedLabelValue = "ai-models"
)

var (
	lvmVolumeGroupSetGVK = schema.GroupVersionKind{
		Group:   "storage.deckhouse.io",
		Version: "v1alpha1",
		Kind:    "LVMVolumeGroupSet",
	}
	lvmVolumeGroupGVK = schema.GroupVersionKind{
		Group:   "storage.deckhouse.io",
		Version: "v1alpha1",
		Kind:    "LVMVolumeGroup",
	}
	lvmVolumeGroupListGVK = schema.GroupVersionKind{
		Group:   "storage.deckhouse.io",
		Version: "v1alpha1",
		Kind:    "LVMVolumeGroupList",
	}
	localStorageClassGVK = schema.GroupVersionKind{
		Group:   "storage.deckhouse.io",
		Version: "v1alpha1",
		Kind:    "LocalStorageClass",
	}
)

func NewLVMVolumeGroupSet(name string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(lvmVolumeGroupSetGVK)
	object.SetName(name)
	return object
}

func NewLVMVolumeGroup(name string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(lvmVolumeGroupGVK)
	object.SetName(name)
	return object
}

func NewLVMVolumeGroupList() *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(lvmVolumeGroupListGVK)
	return list
}

func NewLocalStorageClass(name string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(localStorageClassGVK)
	object.SetName(name)
	return object
}
