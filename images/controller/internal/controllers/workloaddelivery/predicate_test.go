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

package workloaddelivery

import (
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	corev1 "k8s.io/api/core/v1"
)

func TestWorkloadDeliveryInterest(t *testing.T) {
	t.Parallel()

	options := modeldelivery.ServiceOptions{Render: modeldelivery.Options{}}

	t.Run("annotated workload is interesting", func(t *testing.T) {
		t.Parallel()

		workload := annotatedDeployment(map[string]string{ModelAnnotation: "gemma"}, 1, corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		})
		if !workloadDeliveryInterest(workload, options) {
			t.Fatal("expected annotated workload to pass event filter")
		}
	})

	t.Run("managed workload without top-level annotations is interesting", func(t *testing.T) {
		t.Parallel()

		workload := annotatedDeployment(nil, 1, corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		})
		workload.Spec.Template.Annotations = map[string]string{modeldelivery.ResolvedDigestAnnotation: testDigest}
		if !workloadDeliveryInterest(workload, options) {
			t.Fatal("expected managed workload to pass event filter")
		}
	})

	t.Run("module namespace workload is ignored", func(t *testing.T) {
		t.Parallel()

		workload := annotatedDeployment(map[string]string{ModelAnnotation: "gemma"}, 1, corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		})
		workload.Namespace = testRegistryNamespace
		workload.Spec.Template.Annotations = map[string]string{modeldelivery.ResolvedDigestAnnotation: testDigest}
		options := defaultServiceOptions()
		if workloadDeliveryInterest(workload, options) {
			t.Fatal("did not expect module namespace workload to pass event filter")
		}
	})

	t.Run("unmanaged workload without annotations is ignored", func(t *testing.T) {
		t.Parallel()

		workload := annotatedDeployment(nil, 1, corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		})
		if workloadDeliveryInterest(workload, options) {
			t.Fatal("did not expect unannotated unmanaged workload to pass event filter")
		}
	})
}
