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
	"io"
	"log/slog"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func gatherMetrics(t *testing.T, objects ...client.Object) []*dto.MetricFamily {
	t.Helper()

	scheme := testkit.NewScheme(t)
	reader := testkit.NewFakeClient(t, scheme, nil, objects...)
	registry := prometheus.NewPedanticRegistry()
	NewCollector(reader, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(registry)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	return families
}

func assertGaugeValue(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string, want float64) {
	t.Helper()

	metric := findMetric(t, families, name, labels)
	if got := metric.GetGauge().GetValue(); got != want {
		t.Fatalf("metric %s labels=%v value = %v, want %v", name, labels, got, want)
	}
}

func findMetric(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string) *dto.Metric {
	t.Helper()

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if hasExactLabels(metric, labels) {
				return metric
			}
		}
	}

	t.Fatalf("metric %s with labels %v not found", name, labels)
	return nil
}

func hasExactLabels(metric *dto.Metric, expected map[string]string) bool {
	if len(metric.GetLabel()) != len(expected) {
		return false
	}

	for _, label := range metric.GetLabel() {
		value, ok := expected[label.GetName()]
		if !ok || value != label.GetValue() {
			return false
		}
	}

	return true
}
