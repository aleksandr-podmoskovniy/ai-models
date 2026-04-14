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

import (
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
)

var phaseValues = []modelsv1alpha1.ModelPhase{
	modelsv1alpha1.ModelPhasePending,
	modelsv1alpha1.ModelPhaseWaitForUpload,
	modelsv1alpha1.ModelPhasePublishing,
	modelsv1alpha1.ModelPhaseReady,
	modelsv1alpha1.ModelPhaseFailed,
	modelsv1alpha1.ModelPhaseDeleting,
}

func collectorDescs() []*prometheus.Desc {
	return []*prometheus.Desc{
		modelStatusPhaseMetric.desc,
		clusterModelStatusPhaseMetric.desc,
		modelReadyMetric.desc,
		clusterModelReadyMetric.desc,
		modelValidatedMetric.desc,
		clusterModelValidatedMetric.desc,
		modelInfoMetric.desc,
		clusterModelInfoMetric.desc,
		modelArtifactSizeMetric.desc,
		clusterModelArtifactSizeMetric.desc,
	}
}

func (c *Collector) report(ch chan<- prometheus.Metric, metric *objectMetric) {
	if metric == nil {
		return
	}

	c.reportPhaseMetrics(ch, metric)
	c.reportObjectMetrics(ch, metric)
}

func (c *Collector) reportPhaseMetrics(ch chan<- prometheus.Metric, metric *objectMetric) {
	for _, phase := range phaseValues {
		value := boolFloat64(metric.phase == phase)
		switch metric.kind {
		case modelObjectKind:
			ch <- prometheus.MustNewConstMetric(
				modelStatusPhaseMetric.desc,
				prometheus.GaugeValue,
				value,
				metric.name,
				metric.namespace,
				metric.uid,
				string(phase),
				metric.sourceType,
			)
		case clusterModelObjectKind:
			ch <- prometheus.MustNewConstMetric(
				clusterModelStatusPhaseMetric.desc,
				prometheus.GaugeValue,
				value,
				metric.name,
				metric.uid,
				string(phase),
				metric.sourceType,
			)
		}
	}
}

func (c *Collector) reportObjectMetrics(ch chan<- prometheus.Metric, metric *objectMetric) {
	switch metric.kind {
	case modelObjectKind:
		ch <- prometheus.MustNewConstMetric(
			modelReadyMetric.desc,
			prometheus.GaugeValue,
			boolFloat64(metric.ready),
			metric.name,
			metric.namespace,
			metric.uid,
		)
		ch <- prometheus.MustNewConstMetric(
			modelValidatedMetric.desc,
			prometheus.GaugeValue,
			boolFloat64(metric.validated),
			metric.name,
			metric.namespace,
			metric.uid,
		)
		ch <- prometheus.MustNewConstMetric(
			modelInfoMetric.desc,
			prometheus.GaugeValue,
			1,
			metric.name,
			metric.namespace,
			metric.uid,
			metric.sourceType,
			metric.format,
			metric.task,
			metric.framework,
			metric.artifactKind,
		)
		ch <- prometheus.MustNewConstMetric(
			modelArtifactSizeMetric.desc,
			prometheus.GaugeValue,
			metric.artifactSizeByte,
			metric.name,
			metric.namespace,
			metric.uid,
		)
	case clusterModelObjectKind:
		ch <- prometheus.MustNewConstMetric(
			clusterModelReadyMetric.desc,
			prometheus.GaugeValue,
			boolFloat64(metric.ready),
			metric.name,
			metric.uid,
		)
		ch <- prometheus.MustNewConstMetric(
			clusterModelValidatedMetric.desc,
			prometheus.GaugeValue,
			boolFloat64(metric.validated),
			metric.name,
			metric.uid,
		)
		ch <- prometheus.MustNewConstMetric(
			clusterModelInfoMetric.desc,
			prometheus.GaugeValue,
			1,
			metric.name,
			metric.uid,
			metric.sourceType,
			metric.format,
			metric.task,
			metric.framework,
			metric.artifactKind,
		)
		ch <- prometheus.MustNewConstMetric(
			clusterModelArtifactSizeMetric.desc,
			prometheus.GaugeValue,
			metric.artifactSizeByte,
			metric.name,
			metric.uid,
		)
	}
}

func boolFloat64(value bool) float64 {
	if value {
		return 1
	}

	return 0
}
