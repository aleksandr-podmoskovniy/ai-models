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

func TestDesiredLVMVolumeGroupSetBuildsManagedThinPoolSpec(t *testing.T) {
	t.Parallel()

	object := DesiredLVMVolumeGroupSet(LVMVolumeGroupSetSpec{
		Name:                  "ai-models-node-cache",
		MaxSize:               "200Gi",
		ThinPoolName:          "model-cache",
		VolumeGroupNameOnNode: "ai-models-cache",
		NodeSelectorLabels: map[string]string{
			"node-role.kubernetes.io/worker": "",
		},
		BlockDeviceMatchLabels: map[string]string{
			"status.blockdevice.storage.deckhouse.io/model": "nvme",
		},
	})

	if got := object.GetLabels()[ManagedLabelKey]; got != ManagedLabelValue {
		t.Fatalf("managed label = %q, want %q", got, ManagedLabelValue)
	}
	actualVG, _, _ := unstructured.NestedString(object.Object, "spec", "lvmVolumeGroupTemplate", "actualVGNameOnTheNode")
	if actualVG != "ai-models-cache" {
		t.Fatalf("actualVGNameOnTheNode = %q", actualVG)
	}
	thinPools, _, _ := unstructured.NestedSlice(object.Object, "spec", "lvmVolumeGroupTemplate", "thinPools")
	pool := thinPools[0].(map[string]any)
	if pool["name"] != "model-cache" {
		t.Fatalf("thin pool name = %#v", pool["name"])
	}
	if pool["size"] != "200Gi" {
		t.Fatalf("thin pool size = %#v", pool["size"])
	}
}
