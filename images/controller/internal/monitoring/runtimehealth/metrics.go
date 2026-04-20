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
	}
}
