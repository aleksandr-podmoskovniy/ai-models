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

package collectorhealth

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestStateReportsSuccessAndRetainsLastSuccessAfterFailure(t *testing.T) {
	started := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	firstFinished := started.Add(2 * time.Second)
	secondFinished := started.Add(5 * time.Second)
	state := New("catalog-state")

	state.now = func() time.Time { return firstFinished }
	first := reportOnce(t, state, started, true)
	assertMetric(t, first[0], 1, map[string]string{"collector": "catalog-state"})
	assertMetric(t, first[1], 2, map[string]string{"collector": "catalog-state"})
	assertMetric(t, first[2], float64(firstFinished.UnixNano())/float64(time.Second), map[string]string{"collector": "catalog-state"})

	state.now = func() time.Time { return secondFinished }
	second := reportOnce(t, state, started, false)
	assertMetric(t, second[0], 0, map[string]string{"collector": "catalog-state"})
	assertMetric(t, second[1], 5, map[string]string{"collector": "catalog-state"})
	assertMetric(t, second[2], float64(firstFinished.UnixNano())/float64(time.Second), map[string]string{"collector": "catalog-state"})
}

func TestSharedDescriptorsAllowMultipleCollectors(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(
		testCollector{name: "catalog-state"},
		testCollector{name: "runtime-health"},
	)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if len(families) != descriptorCount {
		t.Fatalf("metric family count = %d, want %d", len(families), descriptorCount)
	}
}

type testCollector struct {
	name string
}

func (c testCollector) Describe(ch chan<- *prometheus.Desc) {
	New(c.name).Describe(ch)
}

func (c testCollector) Collect(ch chan<- prometheus.Metric) {
	state := New(c.name)
	now := time.Date(2026, 4, 28, 10, 0, 1, 0, time.UTC)
	state.now = func() time.Time { return now }
	state.Report(ch, now.Add(-time.Second), true)
}

func reportOnce(t *testing.T, state *State, started time.Time, success bool) []prometheus.Metric {
	t.Helper()

	ch := make(chan prometheus.Metric, descriptorCount)
	state.Report(ch, started, success)
	close(ch)

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	if len(metrics) != descriptorCount {
		t.Fatalf("metric count = %d, want %d", len(metrics), descriptorCount)
	}
	return metrics
}

const descriptorCount = 3

func assertMetric(t *testing.T, metric prometheus.Metric, want float64, labels map[string]string) {
	t.Helper()

	var dtoMetric dto.Metric
	if err := metric.Write(&dtoMetric); err != nil {
		t.Fatalf("metric.Write() error = %v", err)
	}
	if got := dtoMetric.GetGauge().GetValue(); got != want {
		t.Fatalf("metric value = %v, want %v", got, want)
	}
	if !hasLabels(&dtoMetric, labels) {
		t.Fatalf("metric labels = %v, want %v", dtoMetric.GetLabel(), labels)
	}
}

func hasLabels(metric *dto.Metric, expected map[string]string) bool {
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
