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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDesiredLocalStorageClassBuildsThinManagedSpec(t *testing.T) {
	t.Parallel()

	object := DesiredLocalStorageClass(LocalStorageClassSpec{
		Name:         "ai-models-node-cache",
		ThinPoolName: "model-cache",
		LVGNames:     []string{"lvg-a", "lvg-b"},
	})

	lvgs, _, _ := unstructured.NestedSlice(object.Object, "spec", "lvm", "lvmVolumeGroups")
	if len(lvgs) != 2 {
		t.Fatalf("lvmVolumeGroups len = %d, want 2", len(lvgs))
	}
	first := lvgs[0].(map[string]any)
	thin := first["thin"].(map[string]any)
	if thin["poolName"] != "model-cache" {
		t.Fatalf("thin pool name = %#v", thin["poolName"])
	}
	kind, _, _ := unstructured.NestedString(object.Object, "spec", "lvm", "type")
	if kind != "Thin" {
		t.Fatalf("storage class type = %q, want Thin", kind)
	}
}
