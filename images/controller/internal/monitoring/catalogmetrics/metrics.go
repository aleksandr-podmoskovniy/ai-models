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

package catalogmetrics

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

func modelLabels(extra ...string) []string {
	return append([]string{"name", "namespace", "uid"}, extra...)
}

func clusterModelLabels(extra ...string) []string {
	return append([]string{"name", "uid"}, extra...)
}

var (
	modelStatusPhaseMetric = newMetricInfo(
		"model_status_phase",
		"The Model current phase.",
		modelLabels("phase", "source_type")...,
	)
	clusterModelStatusPhaseMetric = newMetricInfo(
		"clustermodel_status_phase",
		"The ClusterModel current phase.",
		clusterModelLabels("phase", "source_type")...,
	)
	modelReadyMetric = newMetricInfo(
		"model_ready",
		"Whether the Model is ready for consumption.",
		modelLabels()...,
	)
	clusterModelReadyMetric = newMetricInfo(
		"clustermodel_ready",
		"Whether the ClusterModel is ready for consumption.",
		clusterModelLabels()...,
	)
	modelConditionMetric = newMetricInfo(
		"model_condition",
		"The current Model condition status and reason projected from status.conditions.",
		modelLabels("type", "status", "reason")...,
	)
	clusterModelConditionMetric = newMetricInfo(
		"clustermodel_condition",
		"The current ClusterModel condition status and reason projected from status.conditions.",
		clusterModelLabels("type", "status", "reason")...,
	)
	modelInfoMetric = newMetricInfo(
		"model_info",
		"Static public Model catalog information.",
		modelLabels("resolved_source_type", "format", "task", "artifact_kind")...,
	)
	clusterModelInfoMetric = newMetricInfo(
		"clustermodel_info",
		"Static public ClusterModel catalog information.",
		clusterModelLabels("resolved_source_type", "format", "task", "artifact_kind")...,
	)
	modelArtifactSizeMetric = newMetricInfo(
		"model_artifact_size_bytes",
		"The published Model artifact size in bytes.",
		modelLabels()...,
	)
	clusterModelArtifactSizeMetric = newMetricInfo(
		"clustermodel_artifact_size_bytes",
		"The published ClusterModel artifact size in bytes.",
		clusterModelLabels()...,
	)
)
