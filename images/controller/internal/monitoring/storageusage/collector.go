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

package storageusage

import (
	"context"
	"log/slog"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageaccounting"
	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
	"github.com/prometheus/client_golang/prometheus"
)

const collectorName = "storage-usage"

type Collector struct {
	store  *storageaccounting.Store
	logger *slog.Logger
}

func SetupCollector(store *storageaccounting.Store, registerer prometheus.Registerer, logger *slog.Logger) {
	NewCollector(store, logger).Register(registerer)
}

func NewCollector(store *storageaccounting.Store, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}
	return &Collector{
		store:  store,
		logger: logger.With(slog.String("collector", collectorName)),
	}
}

func (c *Collector) Register(registerer prometheus.Registerer) {
	registerer.MustRegister(c)
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- capacityKnownMetric
	ch <- limitBytesMetric
	ch <- usedBytesMetric
	ch <- reservedBytesMetric
	ch <- availableBytesMetric
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usage, err := c.usage(ctx)
	if err != nil {
		c.logger.Error("failed to collect storage usage", slog.Any("error", err))
		return
	}
	ch <- prometheus.MustNewConstMetric(capacityKnownMetric, prometheus.GaugeValue, boolFloat64(usage.CapacityKnown))
	ch <- prometheus.MustNewConstMetric(limitBytesMetric, prometheus.GaugeValue, float64(usage.LimitBytes))
	ch <- prometheus.MustNewConstMetric(usedBytesMetric, prometheus.GaugeValue, float64(usage.UsedBytes))
	ch <- prometheus.MustNewConstMetric(reservedBytesMetric, prometheus.GaugeValue, float64(usage.ReservedBytes))
	ch <- prometheus.MustNewConstMetric(availableBytesMetric, prometheus.GaugeValue, float64(usage.AvailableBytes))
}

func (c *Collector) usage(ctx context.Context) (storagecapacity.Usage, error) {
	if c.store == nil {
		return storagecapacity.Usage{}, nil
	}
	return c.store.Usage(ctx)
}

func boolFloat64(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
