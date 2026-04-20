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
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const collectorName = "runtime-health"

type Options struct {
	RuntimeNamespace   string
	NodeSelectorLabels map[string]string
}

func SetupCollector(reader client.Reader, registerer prometheus.Registerer, logger *slog.Logger, options Options) {
	NewCollector(reader, logger, options).Register(registerer)
}

func NewCollector(reader client.Reader, logger *slog.Logger, options Options) *Collector {
	if logger == nil {
		logger = slog.Default()
	}
	options = normalizeOptions(options)

	return &Collector{
		reader:             reader,
		logger:             logger.With(slog.String("collector", collectorName)),
		runtimeNamespace:   options.RuntimeNamespace,
		nodeSelectorLabels: options.NodeSelectorLabels,
	}
}

type Collector struct {
	reader             client.Reader
	logger             *slog.Logger
	runtimeNamespace   string
	nodeSelectorLabels map[string]string
}

func (c *Collector) Register(registerer prometheus.Registerer) {
	registerer.MustRegister(c)
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range collectorDescs() {
		ch <- desc
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pods, err := c.listNodeCacheRuntimePods(ctx)
	if err != nil {
		c.logger.Error("failed to list node-cache runtime Pods for metrics collection", slog.Any("error", err))
	}
	pvcs, err := c.listNodeCacheRuntimePVCs(ctx)
	if err != nil {
		c.logger.Error("failed to list node-cache runtime PVCs for metrics collection", slog.Any("error", err))
	}
	selectedNodes, err := c.listSelectedNodes(ctx)
	if err != nil {
		c.logger.Error("failed to list selected nodes for runtime-health metrics collection", slog.Any("error", err))
	}
	if err := c.collectManagedWorkloadDelivery(ctx, ch); err != nil {
		c.logger.Error("failed to list managed workloads for runtime-health metrics collection", slog.Any("error", err))
	}

	reportNodeCacheRuntimeSummary(ch, strings.TrimSpace(c.runtimeNamespace), selectedNodes, pods, pvcs)
	for i := range pods {
		reportNodeCacheRuntimePod(ch, &pods[i])
	}
	for i := range pvcs {
		reportNodeCacheRuntimePVC(ch, &pvcs[i])
	}
}

func normalizeOptions(options Options) Options {
	options.RuntimeNamespace = strings.TrimSpace(options.RuntimeNamespace)
	if len(options.NodeSelectorLabels) == 0 {
		options.NodeSelectorLabels = nil
		return options
	}
	normalized := make(map[string]string, len(options.NodeSelectorLabels))
	for key, value := range options.NodeSelectorLabels {
		normalized[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	options.NodeSelectorLabels = normalized
	return options
}
