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

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

type LocalStorageClassSpec struct {
	Name         string
	ThinPoolName string
	LVGNames     []string
}

func DesiredLocalStorageClass(spec LocalStorageClassSpec) *unstructured.Unstructured {
	object := NewLocalStorageClass(spec.Name)
	object.SetLabels(map[string]string{
		ManagedLabelKey: ManagedLabelValue,
	})

	lvgs := make([]any, 0, len(spec.LVGNames))
	for _, name := range spec.LVGNames {
		lvgs = append(lvgs, map[string]any{
			"name": name,
			"thin": map[string]any{
				"poolName": spec.ThinPoolName,
			},
		})
	}

	object.Object["spec"] = map[string]any{
		"lvm": map[string]any{
			"lvmVolumeGroups": lvgs,
			"type":            "Thin",
		},
		"reclaimPolicy":     "Delete",
		"volumeBindingMode": "WaitForFirstConsumer",
	}
	return object
}
