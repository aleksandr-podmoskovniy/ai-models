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

	"github.com/deckhouse/ai-models/controller/internal/workloaddelivery"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCollectorReportsManagedWorkloadDeliveryCounts(t *testing.T) {
	t.Parallel()

	families := gatherMetrics(t, Options{},
		newManagedDeliveryDeployment(
			"team-a",
			"deployment-a",
			string(workloaddelivery.DeliveryModeMaterializeBridge),
			string(workloaddelivery.DeliveryReasonManagedBridgeVolume),
		),
		newManagedDeliveryDeployment(
			"team-a",
			"deployment-b",
			string(workloaddelivery.DeliveryModeMaterializeBridge),
			string(workloaddelivery.DeliveryReasonManagedBridgeVolume),
		),
		newManagedDeliveryStatefulSet(
			"team-b",
			"statefulset-a",
			string(workloaddelivery.DeliveryModeSharedPVCBridge),
			string(workloaddelivery.DeliveryReasonWorkloadSharedPersistentVolume),
		),
		newManagedDeliveryCronJob(
			"team-c",
			"cronjob-a",
			string(workloaddelivery.DeliveryModeMaterializeBridge),
			string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
		),
		&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "team-a",
				Name:      "daemonset-unmanaged",
			},
		},
	)

	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_workloads_managed", map[string]string{
		"namespace":       "team-a",
		"kind":            "Deployment",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonManagedBridgeVolume),
	}, 2)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_workloads_managed", map[string]string{
		"namespace":       "team-b",
		"kind":            "StatefulSet",
		"delivery_mode":   string(workloaddelivery.DeliveryModeSharedPVCBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadSharedPersistentVolume),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_workloads_managed", map[string]string{
		"namespace":       "team-c",
		"kind":            "CronJob",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
	}, 1)
	assertMetricAbsent(t, families, "d8_ai_models_workload_delivery_workloads_managed", map[string]string{
		"namespace":       "team-a",
		"kind":            "DaemonSet",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonManagedBridgeVolume),
	})
}

func TestCollectorGroupsIncompleteManagedWorkloadStateUnderUnknown(t *testing.T) {
	t.Parallel()

	families := gatherMetrics(t, Options{},
		newManagedDeliveryDaemonSet("team-a", "daemonset-a", string(workloaddelivery.DeliveryModeSharedDirect), ""),
		newManagedDeliveryCronJob("team-b", "cronjob-a", "", ""),
	)

	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_workloads_managed", map[string]string{
		"namespace":       "team-a",
		"kind":            "DaemonSet",
		"delivery_mode":   string(workloaddelivery.DeliveryModeSharedDirect),
		"delivery_reason": unknownDeliveryReason,
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_workloads_managed", map[string]string{
		"namespace":       "team-b",
		"kind":            "CronJob",
		"delivery_mode":   unknownDeliveryMode,
		"delivery_reason": unknownDeliveryReason,
	}, 1)
}

func TestCollectorReportsManagedWorkloadPodState(t *testing.T) {
	t.Parallel()

	families := gatherMetrics(t, Options{},
		newManagedDeliveryPod(
			"team-a",
			"runtime-ready",
			string(workloaddelivery.DeliveryModeMaterializeBridge),
			string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
			true,
			corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 0,
					Reason:   "Completed",
				},
			},
		),
		newManagedDeliveryPod(
			"team-a",
			"runtime-pulling",
			string(workloaddelivery.DeliveryModeMaterializeBridge),
			string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
			false,
			corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
			},
		),
		newManagedDeliveryPod(
			"team-a",
			"runtime-copying",
			string(workloaddelivery.DeliveryModeMaterializeBridge),
			string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
			false,
			corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{},
			},
		),
		newManagedDeliveryPod(
			"team-a",
			"runtime-failed",
			string(workloaddelivery.DeliveryModeMaterializeBridge),
			string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
			false,
			corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 1,
					Reason:   "Error",
				},
			},
		),
		newManagedDeliveryPod(
			"team-b",
			"runtime-shared-pvc",
			string(workloaddelivery.DeliveryModeSharedPVCBridge),
			string(workloaddelivery.DeliveryReasonWorkloadSharedPersistentVolume),
			false,
			corev1.ContainerState{},
		),
	)

	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_pods_managed", map[string]string{
		"namespace":       "team-a",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
	}, 4)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_pods_ready", map[string]string{
		"namespace":       "team-a",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_init_state", map[string]string{
		"namespace":       "team-a",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
		"state":           "Succeeded",
		"reason":          "Completed",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_init_state", map[string]string{
		"namespace":       "team-a",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
		"state":           "Waiting",
		"reason":          "ImagePullBackOff",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_init_state", map[string]string{
		"namespace":       "team-a",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
		"state":           "Running",
		"reason":          "",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_init_state", map[string]string{
		"namespace":       "team-a",
		"delivery_mode":   string(workloaddelivery.DeliveryModeMaterializeBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadCacheVolume),
		"state":           "Failed",
		"reason":          "Error",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_workload_delivery_pods_managed", map[string]string{
		"namespace":       "team-b",
		"delivery_mode":   string(workloaddelivery.DeliveryModeSharedPVCBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadSharedPersistentVolume),
	}, 1)
	assertMetricAbsent(t, families, "d8_ai_models_workload_delivery_init_state", map[string]string{
		"namespace":       "team-b",
		"delivery_mode":   string(workloaddelivery.DeliveryModeSharedPVCBridge),
		"delivery_reason": string(workloaddelivery.DeliveryReasonWorkloadSharedPersistentVolume),
		"state":           "Waiting",
		"reason":          "ImagePullBackOff",
	})
}

func newManagedDeliveryDeployment(namespace, name, mode, reason string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.DeploymentSpec{
			Template: managedDeliveryPodTemplate(mode, reason),
		},
	}
}

func newManagedDeliveryStatefulSet(namespace, name, mode, reason string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.StatefulSetSpec{
			Template: managedDeliveryPodTemplate(mode, reason),
		},
	}
}

func newManagedDeliveryDaemonSet(namespace, name, mode, reason string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.DaemonSetSpec{
			Template: managedDeliveryPodTemplate(mode, reason),
		},
	}
}

func newManagedDeliveryCronJob(namespace, name, mode, reason string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: managedDeliveryPodTemplate(mode, reason),
				},
			},
		},
	}
}

func managedDeliveryPodTemplate(mode, reason string) corev1.PodTemplateSpec {
	annotations := map[string]string{
		workloaddelivery.ResolvedDigestAnnotation: "sha256:1234",
	}
	if mode != "" {
		annotations[workloaddelivery.ResolvedDeliveryModeAnnotation] = mode
	}
	if reason != "" {
		annotations[workloaddelivery.ResolvedDeliveryReasonAnnotation] = reason
	}

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
		},
	}
}

func newManagedDeliveryPod(
	namespace,
	name,
	mode,
	reason string,
	ready bool,
	initState corev1.ContainerState,
) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: managedDeliveryPodTemplate(mode, reason).Annotations,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	if ready {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		})
	}
	if initState.Waiting != nil || initState.Running != nil || initState.Terminated != nil {
		pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{
			Name:  workloaddelivery.DefaultMaterializerInitContainerName,
			State: initState,
		})
	}
	return pod
}
