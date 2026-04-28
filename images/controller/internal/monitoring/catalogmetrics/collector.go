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

	"github.com/deckhouse/ai-models/controller/internal/monitoring/collectorhealth"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const collectorName = "catalog-state"

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
		health: collectorhealth.New(collectorName),
	}
}

type Collector struct {
	reader client.Reader
	logger *slog.Logger
	health *collectorhealth.State
}

func (c *Collector) Register(registerer prometheus.Registerer) {
	registerer.MustRegister(c)
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range collectorDescs() {
		ch <- desc
	}
	c.health.Describe(ch)
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	success := true
	if err := c.collectModels(ctx, ch); err != nil {
		success = false
		c.logger.Error("failed to list Models for metrics collection", slog.Any("error", err))
	}
	if err := c.collectClusterModels(ctx, ch); err != nil {
		success = false
		c.logger.Error("failed to list ClusterModels for metrics collection", slog.Any("error", err))
	}
	c.health.Report(ch, startedAt, success)
}
