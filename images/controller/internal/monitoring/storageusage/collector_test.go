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
	"io"
	"log/slog"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageaccounting"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCollectorReportsUnknownCapacityWhenStoreDisabled(t *testing.T) {
	registry := prometheus.NewRegistry()
	SetupCollector(nil, registry, nil)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	assertGaugeValue(t, families, "d8_ai_models_storage_backend_capacity_known", nil, 0)
	assertGaugeValue(t, families, "d8_ai_models_storage_backend_limit_bytes", nil, 0)
	assertGaugeValue(t, families, "d8_ai_models_collector_up", map[string]string{"collector": collectorName}, 1)
}

func TestCollectorReportsUnhealthyWhenLedgerReadFails(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      storageaccounting.DefaultSecretName,
			Namespace: "d8-ai-models",
		},
		Data: map[string][]byte{"ledger.json": []byte("not-json")},
	}
	store, err := storageaccounting.New(fake.NewClientBuilder().WithObjects(secret).Build(), storageaccounting.Options{
		Namespace:  "d8-ai-models",
		LimitBytes: 1,
	})
	if err != nil {
		t.Fatalf("storageaccounting.New() error = %v", err)
	}

	registry := prometheus.NewPedanticRegistry()
	NewCollector(store, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(registry)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	assertGaugeValue(t, families, "d8_ai_models_collector_up", map[string]string{"collector": collectorName}, 0)
}

func assertGaugeValue(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string, want float64) {
	t.Helper()

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if !hasExactLabels(metric, labels) {
				continue
			}
			if got := metric.GetGauge().GetValue(); got != want {
				t.Fatalf("metric %s labels=%v value = %v, want %v", name, labels, got, want)
			}
			return
		}
	}
	t.Fatalf("metric %s labels=%v not found", name, labels)
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
