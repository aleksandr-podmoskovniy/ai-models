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

import "testing"

func TestOptionsValidateEnabledContract(t *testing.T) {
	t.Parallel()

	options := Options{
		Enabled:               true,
		MaxSize:               "200Gi",
		StorageClassName:      "ai-models-node-cache",
		VolumeGroupSetName:    "ai-models-node-cache",
		VolumeGroupNameOnNode: "ai-models-cache",
		ThinPoolName:          "model-cache",
		NodeSelectorLabels: map[string]string{
			"node-role.kubernetes.io/worker": "",
		},
		BlockDeviceMatchLabels: map[string]string{
			"status.blockdevice.storage.deckhouse.io/model": "nvme",
		},
	}
	if err := options.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestOptionsValidateRejectsEmptySelectors(t *testing.T) {
	t.Parallel()

	options := Options{Enabled: true, MaxSize: "200Gi", StorageClassName: "sc", VolumeGroupSetName: "set", VolumeGroupNameOnNode: "vg", ThinPoolName: "pool"}
	if err := options.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
