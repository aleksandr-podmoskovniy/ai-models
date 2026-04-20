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

package nodecacheruntime

import "testing"

func TestDesiredPVC(t *testing.T) {
	t.Parallel()

	pvc, err := DesiredPVC(RuntimeSpec{
		Namespace:        "d8-ai-models",
		NodeName:         "worker-a",
		StorageClassName: "ai-models-node-cache",
		SharedVolumeSize: "64Gi",
	})
	if err != nil {
		t.Fatalf("DesiredPVC() error = %v", err)
	}

	if pvc.Name != "ai-models-node-cache-worker-a" {
		t.Fatalf("unexpected PVC name %q", pvc.Name)
	}
	if pvc.Namespace != "d8-ai-models" {
		t.Fatalf("unexpected PVC namespace %q", pvc.Namespace)
	}
	if pvc.Annotations[NodeNameAnnotationKey] != "worker-a" {
		t.Fatalf("unexpected node annotation %#v", pvc.Annotations)
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "ai-models-node-cache" {
		t.Fatalf("unexpected storageClassName %#v", pvc.Spec.StorageClassName)
	}
	if got := pvc.Spec.Resources.Requests.Storage().String(); got != "64Gi" {
		t.Fatalf("unexpected storage request %q", got)
	}
}

func TestDesiredPVCRejectsInvalidSize(t *testing.T) {
	t.Parallel()

	if _, err := DesiredPVC(RuntimeSpec{
		Namespace:        "d8-ai-models",
		NodeName:         "worker-a",
		StorageClassName: "ai-models-node-cache",
		SharedVolumeSize: "not-a-quantity",
	}); err == nil {
		t.Fatal("expected invalid size error")
	}
}
