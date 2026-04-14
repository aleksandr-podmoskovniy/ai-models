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

package modeldelivery

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

const multiplePodsHint int32 = 2

func HintsForDeployment(workload *appsv1.Deployment) TopologyHints {
	if workload == nil {
		return TopologyHints{ReplicaCount: 1}
	}
	return TopologyHints{ReplicaCount: dereferenceReplicas(workload.Spec.Replicas, 1)}
}

func HintsForStatefulSet(workload *appsv1.StatefulSet) TopologyHints {
	if workload == nil {
		return TopologyHints{ReplicaCount: 1}
	}
	return TopologyHints{
		ReplicaCount: dereferenceReplicas(workload.Spec.Replicas, 1),
		VolumeClaimTemplates: append([]corev1.PersistentVolumeClaim(nil),
			workload.Spec.VolumeClaimTemplates...),
	}
}

func HintsForDaemonSet(workload *appsv1.DaemonSet) TopologyHints {
	if workload == nil {
		return TopologyHints{ReplicaCount: multiplePodsHint}
	}
	return TopologyHints{ReplicaCount: multiplePodsHint}
}

func HintsForJob(workload *batchv1.Job) TopologyHints {
	if workload == nil {
		return TopologyHints{ReplicaCount: 1}
	}
	return TopologyHints{ReplicaCount: dereferenceReplicas(workload.Spec.Parallelism, 1)}
}

func HintsForCronJob(workload *batchv1.CronJob) TopologyHints {
	if workload == nil {
		return TopologyHints{ReplicaCount: 1}
	}
	return TopologyHints{ReplicaCount: dereferenceReplicas(workload.Spec.JobTemplate.Spec.Parallelism, 1)}
}

func dereferenceReplicas(value *int32, fallback int32) int32 {
	if value == nil {
		return fallback
	}
	if *value <= 0 {
		return fallback
	}
	return *value
}
