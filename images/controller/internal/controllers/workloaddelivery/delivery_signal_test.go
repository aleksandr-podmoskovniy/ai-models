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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeliverySignalStateFromTemplateProjectsWorkloadFacingContract(t *testing.T) {
	t.Parallel()

	template := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				modeldelivery.ResolvedDigestAnnotation:         "  sha256:test  ",
				modeldelivery.ResolvedArtifactURIAnnotation:    "  registry.example/gemma@sha256:test ",
				modeldelivery.ResolvedArtifactFamilyAnnotation: " gemma ",
				modeldelivery.ResolvedDeliveryModeAnnotation:   " SharedDirect ",
				modeldelivery.ResolvedDeliveryReasonAnnotation: " NodeSharedRuntimePlane ",
				"other": "ignored",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "runtime",
					Env: []corev1.EnvVar{
						{Name: "OTHER_ENV", Value: "ignored"},
						{Name: modeldelivery.ModelPathEnv, Value: " /data/modelcache/model "},
					},
				},
			},
		},
	}

	got := deliverySignalStateFromTemplate(template)
	want := deliverySignalState{
		Digest:         "sha256:test",
		ArtifactURI:    "registry.example/gemma@sha256:test",
		ArtifactFamily: "gemma",
		ModelPath:      "/data/modelcache/model",
		DeliveryMode:   "SharedDirect",
		DeliveryReason: "NodeSharedRuntimePlane",
	}
	if got != want {
		t.Fatalf("deliverySignalStateFromTemplate() = %#v, want %#v", got, want)
	}
}

func TestDeliverySignalStateFromTemplateReturnsEmptyStateForNilTemplate(t *testing.T) {
	t.Parallel()

	if got := deliverySignalStateFromTemplate(nil); !got.empty() {
		t.Fatalf("deliverySignalStateFromTemplate(nil) = %#v, want empty state", got)
	}
}
