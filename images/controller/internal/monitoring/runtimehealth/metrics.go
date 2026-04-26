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

import "github.com/prometheus/client_golang/prometheus"

const metricNamespace = "d8_ai_models"

type metricInfo struct {
	desc *prometheus.Desc
}

func newMetricInfo(name, help string, labels ...string) metricInfo {
	return metricInfo{
		desc: prometheus.NewDesc(
			prometheus.BuildFQName(metricNamespace, "", name),
			help,
			labels,
			nil,
		),
	}
}

var (
	nodeCacheRuntimeNodesDesiredMetric = newMetricInfo(
		"node_cache_runtime_nodes_desired",
		"Number of selected nodes that should run the managed node-cache runtime plane.",
		"namespace",
	)
	nodeCacheRuntimePodsManagedMetric = newMetricInfo(
		"node_cache_runtime_pods_managed",
		"Number of managed node-cache runtime Pods in the runtime namespace.",
		"namespace",
	)
	nodeCacheRuntimePodsReadyMetric = newMetricInfo(
		"node_cache_runtime_pods_ready",
		"Number of ready managed node-cache runtime Pods in the runtime namespace.",
		"namespace",
	)
	nodeCacheRuntimePVCsManagedMetric = newMetricInfo(
		"node_cache_runtime_pvcs_managed",
		"Number of managed node-cache runtime PVCs in the runtime namespace.",
		"namespace",
	)
	nodeCacheRuntimePVCsBoundMetric = newMetricInfo(
		"node_cache_runtime_pvcs_bound",
		"Number of bound managed node-cache runtime PVCs in the runtime namespace.",
		"namespace",
	)
	nodeCacheRuntimePodPhaseMetric = newMetricInfo(
		"node_cache_runtime_pod_phase",
		"The managed node-cache runtime Pod current phase.",
		"namespace", "name", "node", "phase",
	)
	nodeCacheRuntimePodReadyMetric = newMetricInfo(
		"node_cache_runtime_pod_ready",
		"Whether the managed node-cache runtime Pod is ready.",
		"namespace", "name", "node",
	)
	nodeCacheRuntimePVCBoundMetric = newMetricInfo(
		"node_cache_runtime_pvc_bound",
		"Whether the managed node-cache runtime PVC is bound.",
		"namespace", "name", "node", "storage_class",
	)
	nodeCacheRuntimePVCRequestedBytesMetric = newMetricInfo(
		"node_cache_runtime_pvc_requested_bytes",
		"Requested storage size of the managed node-cache runtime PVC.",
		"namespace", "name", "node", "storage_class",
	)
	workloadDeliveryWorkloadsManagedMetric = newMetricInfo(
		"workload_delivery_workloads_managed",
		"Number of top-level workloads with applied managed runtime delivery grouped by namespace, kind, delivery mode, and delivery reason.",
		"namespace", "kind", "delivery_mode", "delivery_reason",
	)
	workloadDeliveryPodsManagedMetric = newMetricInfo(
		"workload_delivery_pods_managed",
		"Number of workload Pods carrying managed runtime delivery annotations grouped by namespace, delivery mode, and delivery reason.",
		"namespace", "delivery_mode", "delivery_reason",
	)
	workloadDeliveryPodsReadyMetric = newMetricInfo(
		"workload_delivery_pods_ready",
		"Number of ready workload Pods carrying managed runtime delivery annotations grouped by namespace, delivery mode, and delivery reason.",
		"namespace", "delivery_mode", "delivery_reason",
	)
	workloadDeliveryInitStateMetric = newMetricInfo(
		"workload_delivery_init_state",
		"Number of workload Pods whose managed materialize bridge init container is in the reported state, grouped by namespace, delivery mode, delivery reason, state, and reason.",
		"namespace", "delivery_mode", "delivery_reason", "state", "reason",
	)
	dmcrGCRequestsMetric = newMetricInfo(
		"dmcr_gc_requests",
		"Number of module-private DMCR garbage-collection requests grouped by lifecycle phase.",
		"namespace", "phase",
	)
	dmcrGCRequestAgeSecondsMetric = newMetricInfo(
		"dmcr_gc_request_age_seconds",
		"Age of a module-private DMCR garbage-collection request in its current lifecycle phase.",
		"namespace", "name", "phase",
	)
)

func collectorDescs() []*prometheus.Desc {
	return []*prometheus.Desc{
		nodeCacheRuntimeNodesDesiredMetric.desc,
		nodeCacheRuntimePodsManagedMetric.desc,
		nodeCacheRuntimePodsReadyMetric.desc,
		nodeCacheRuntimePVCsManagedMetric.desc,
		nodeCacheRuntimePVCsBoundMetric.desc,
		nodeCacheRuntimePodPhaseMetric.desc,
		nodeCacheRuntimePodReadyMetric.desc,
		nodeCacheRuntimePVCBoundMetric.desc,
		nodeCacheRuntimePVCRequestedBytesMetric.desc,
		workloadDeliveryWorkloadsManagedMetric.desc,
		workloadDeliveryPodsManagedMetric.desc,
		workloadDeliveryPodsReadyMetric.desc,
		workloadDeliveryInitStateMetric.desc,
		dmcrGCRequestsMetric.desc,
		dmcrGCRequestAgeSecondsMetric.desc,
	}
}
