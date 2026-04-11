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
	"context"
	"log/slog"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const collectorName = "catalog-state"

var phaseValues = []modelsv1alpha1.ModelPhase{
	modelsv1alpha1.ModelPhasePending,
	modelsv1alpha1.ModelPhaseWaitForUpload,
	modelsv1alpha1.ModelPhasePublishing,
	modelsv1alpha1.ModelPhaseReady,
	modelsv1alpha1.ModelPhaseFailed,
	modelsv1alpha1.ModelPhaseDeleting,
}

func SetupCollector(reader client.Reader, registerer prometheus.Registerer, logger *slog.Logger) {
	NewCollector(reader, logger).Register(registerer)
}

func NewCollector(reader client.Reader, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}

	return &Collector{
		reader: reader,
		logger: logger.With(slog.String("collector", collectorName)),
	}
}

type Collector struct {
	reader client.Reader
	logger *slog.Logger
}

func (c *Collector) Register(registerer prometheus.Registerer) {
	registerer.MustRegister(c)
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	descs := []*prometheus.Desc{
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
	for _, desc := range descs {
		ch <- desc
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := c.collectModels(ctx, ch); err != nil {
		c.logger.Error("failed to list Models for metrics collection", slog.Any("error", err))
	}
	if err := c.collectClusterModels(ctx, ch); err != nil {
		c.logger.Error("failed to list ClusterModels for metrics collection", slog.Any("error", err))
	}
}

func (c *Collector) collectModels(ctx context.Context, ch chan<- prometheus.Metric) error {
	var list modelsv1alpha1.ModelList
	if err := c.reader.List(ctx, &list, client.UnsafeDisableDeepCopy); err != nil {
		return err
	}

	for i := range list.Items {
		c.report(ch, newModelMetric(&list.Items[i]))
	}

	return nil
}

func (c *Collector) collectClusterModels(ctx context.Context, ch chan<- prometheus.Metric) error {
	var list modelsv1alpha1.ClusterModelList
	if err := c.reader.List(ctx, &list, client.UnsafeDisableDeepCopy); err != nil {
		return err
	}

	for i := range list.Items {
		c.report(ch, newClusterModelMetric(&list.Items[i]))
	}

	return nil
}

func (c *Collector) report(ch chan<- prometheus.Metric, metric *objectMetric) {
	if metric == nil {
		return
	}

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
