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

package runtimehealth

import (
	"testing"

	k8snodecacheruntime "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestCollectorReportsManagedNodeCacheRuntimeHealthMetrics(t *testing.T) {
	t.Parallel()

	requestedSize := resource.MustParse("64Gi")
	families := gatherMetrics(t, Options{
		RuntimeNamespace: "d8-ai-models",
		NodeSelectorLabels: map[string]string{
			"node-role.deckhouse.io/ai-models-cache": "enabled",
		},
	},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-a",
				Labels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-b",
				Labels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-c",
				Labels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "disabled"},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ai-models-node-cache-runtime-worker-a",
				Namespace: "d8-ai-models",
				Labels: map[string]string{
					k8snodecacheruntime.ManagedLabelKey: k8snodecacheruntime.ManagedLabelValue,
				},
				Annotations: map[string]string{
					k8snodecacheruntime.NodeNameAnnotationKey: "worker-a",
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "worker-a",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ai-models-node-cache-runtime-worker-a-shadow",
				Namespace: "team-a",
				Labels: map[string]string{
					k8snodecacheruntime.ManagedLabelKey: k8snodecacheruntime.ManagedLabelValue,
				},
				Annotations: map[string]string{
					k8snodecacheruntime.NodeNameAnnotationKey: "worker-a",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ai-models-node-cache-worker-a",
				Namespace: "d8-ai-models",
				Labels: map[string]string{
					k8snodecacheruntime.ManagedLabelKey: k8snodecacheruntime.ManagedLabelValue,
				},
				Annotations: map[string]string{
					k8snodecacheruntime.NodeNameAnnotationKey: "worker-a",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.To("ai-models-node-cache"),
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: requestedSize,
					},
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: corev1.ClaimBound,
			},
		},
	)

	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_nodes_desired", map[string]string{
		"namespace": "d8-ai-models",
	}, 2)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pods_managed", map[string]string{
		"namespace": "d8-ai-models",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pods_ready", map[string]string{
		"namespace": "d8-ai-models",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pvcs_managed", map[string]string{
		"namespace": "d8-ai-models",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pvcs_bound", map[string]string{
		"namespace": "d8-ai-models",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pod_phase", map[string]string{
		"namespace": "d8-ai-models",
		"name":      "ai-models-node-cache-runtime-worker-a",
		"node":      "worker-a",
		"phase":     string(corev1.PodRunning),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pod_ready", map[string]string{
		"namespace": "d8-ai-models",
		"name":      "ai-models-node-cache-runtime-worker-a",
		"node":      "worker-a",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pvc_bound", map[string]string{
		"namespace":     "d8-ai-models",
		"name":          "ai-models-node-cache-worker-a",
		"node":          "worker-a",
		"storage_class": "ai-models-node-cache",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pvc_requested_bytes", map[string]string{
		"namespace":     "d8-ai-models",
		"name":          "ai-models-node-cache-worker-a",
		"node":          "worker-a",
		"storage_class": "ai-models-node-cache",
	}, float64(requestedSize.Value()))
	assertMetricAbsent(t, families, "d8_ai_models_node_cache_runtime_pod_ready", map[string]string{
		"namespace": "team-a",
		"name":      "ai-models-node-cache-runtime-worker-a-shadow",
		"node":      "worker-a",
	})
}

func TestCollectorIgnoresUnmanagedResourcesAndReportsUnreadyPendingState(t *testing.T) {
	t.Parallel()

	families := gatherMetrics(t, Options{
		RuntimeNamespace: "d8-ai-models",
		NodeSelectorLabels: map[string]string{
			"node-role.deckhouse.io/ai-models-cache": "enabled",
		},
	},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-b",
				Labels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ai-models-node-cache-runtime-worker-b",
				Namespace: "d8-ai-models",
				Labels: map[string]string{
					k8snodecacheruntime.ManagedLabelKey: k8snodecacheruntime.ManagedLabelValue,
				},
				Annotations: map[string]string{
					k8snodecacheruntime.NodeNameAnnotationKey: "worker-b",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ai-models-node-cache-worker-b",
				Namespace: "d8-ai-models",
				Labels: map[string]string{
					k8snodecacheruntime.ManagedLabelKey: k8snodecacheruntime.ManagedLabelValue,
				},
				Annotations: map[string]string{
					k8snodecacheruntime.NodeNameAnnotationKey: "worker-b",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.To("ai-models-node-cache"),
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: corev1.ClaimPending,
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unmanaged",
				Namespace: "d8-ai-models",
			},
		},
	)

	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_nodes_desired", map[string]string{
		"namespace": "d8-ai-models",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pods_managed", map[string]string{
		"namespace": "d8-ai-models",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pods_ready", map[string]string{
		"namespace": "d8-ai-models",
	}, 0)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pvcs_managed", map[string]string{
		"namespace": "d8-ai-models",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pvcs_bound", map[string]string{
		"namespace": "d8-ai-models",
	}, 0)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pod_ready", map[string]string{
		"namespace": "d8-ai-models",
		"name":      "ai-models-node-cache-runtime-worker-b",
		"node":      "worker-b",
	}, 0)
	assertGaugeValue(t, families, "d8_ai_models_node_cache_runtime_pvc_bound", map[string]string{
		"namespace":     "d8-ai-models",
		"name":          "ai-models-node-cache-worker-b",
		"node":          "worker-b",
		"storage_class": "ai-models-node-cache",
	}, 0)
	assertMetricAbsent(t, families, "d8_ai_models_node_cache_runtime_pod_ready", map[string]string{
		"namespace": "d8-ai-models",
		"name":      "unmanaged",
		"node":      "",
	})
}
