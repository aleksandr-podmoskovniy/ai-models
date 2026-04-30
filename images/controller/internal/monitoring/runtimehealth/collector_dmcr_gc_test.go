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
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errUnexpectedCachedSecretList = errors.New("cached reader must not list secrets")

func TestCollectorReportsDMCRGarbageCollectionRequestLifecycleMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	families := gatherMetricsAt(t, Options{CleanupNamespace: "d8-ai-models"}, now,
		dmcrGCSecret("queued", "d8-ai-models", map[string]string{
			dmcrGCPhaseAnnotationKey:     dmcrGCPhaseQueued,
			dmcrGCRequestedAnnotationKey: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
		}),
		dmcrGCSecret("armed", "d8-ai-models", map[string]string{
			dmcrGCSwitchAnnotationKey:    now.Format(time.RFC3339Nano),
			dmcrGCRequestedAnnotationKey: now.Add(-5 * time.Minute).Format(time.RFC3339Nano),
		}),
		dmcrGCSecret("done", "d8-ai-models", map[string]string{
			dmcrGCPhaseAnnotationKey: dmcrGCPhaseDone,
			dmcrGCCompletedAtKey:     now.Add(-30 * time.Second).Format(time.RFC3339Nano),
		}),
		dmcrGCSecret("unknown", "d8-ai-models", map[string]string{}),
		dmcrGCSecret("other-namespace", "team-a", map[string]string{
			dmcrGCPhaseAnnotationKey: dmcrGCPhaseQueued,
		}),
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "unmanaged", Namespace: "d8-ai-models"}},
	)

	assertGaugeValue(t, families, "d8_ai_models_dmcr_gc_requests", map[string]string{
		"namespace": "d8-ai-models",
		"phase":     dmcrGCPhaseQueued,
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_dmcr_gc_requests", map[string]string{
		"namespace": "d8-ai-models",
		"phase":     dmcrGCPhaseArmed,
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_dmcr_gc_requests", map[string]string{
		"namespace": "d8-ai-models",
		"phase":     dmcrGCPhaseDone,
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_dmcr_gc_requests", map[string]string{
		"namespace": "d8-ai-models",
		"phase":     dmcrGCPhaseUnknown,
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_dmcr_gc_request_age_seconds", map[string]string{
		"namespace": "d8-ai-models",
		"name":      "queued",
		"phase":     dmcrGCPhaseQueued,
	}, 120)
	assertGaugeValue(t, families, "d8_ai_models_dmcr_gc_request_age_seconds", map[string]string{
		"namespace": "d8-ai-models",
		"name":      "armed",
		"phase":     dmcrGCPhaseArmed,
	}, 300)
	assertGaugeValue(t, families, "d8_ai_models_dmcr_gc_request_age_seconds", map[string]string{
		"namespace": "d8-ai-models",
		"name":      "done",
		"phase":     dmcrGCPhaseDone,
	}, 30)
	assertMetricAbsent(t, families, "d8_ai_models_dmcr_gc_requests", map[string]string{
		"namespace": "team-a",
		"phase":     dmcrGCPhaseQueued,
	})
	assertMetricAbsent(t, families, "d8_ai_models_node_cache_runtime_nodes_desired", map[string]string{
		"namespace": "",
	})
	assertGaugeValue(t, families, "d8_ai_models_collector_up", map[string]string{
		"collector": collectorName,
	}, 1)
}

func TestCollectorUsesDedicatedSecretReaderForDMCRGarbageCollectionRequests(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	scheme := testkit.NewScheme(t, appsv1.AddToScheme, batchv1.AddToScheme)
	cachedReader := secretListRejectingReader{Reader: testkit.NewFakeClient(t, scheme, nil)}
	secretReader := testkit.NewFakeClient(t, scheme, nil,
		dmcrGCSecret("queued", "d8-ai-models", map[string]string{
			dmcrGCPhaseAnnotationKey:     dmcrGCPhaseQueued,
			dmcrGCRequestedAnnotationKey: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
		}),
	)
	registry := prometheus.NewPedanticRegistry()
	collector := NewCollector(
		cachedReader,
		secretReader,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Options{CleanupNamespace: "d8-ai-models"},
	)
	collector.now = func() time.Time { return now }
	collector.Register(registry)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	assertGaugeValue(t, families, "d8_ai_models_dmcr_gc_requests", map[string]string{
		"namespace": "d8-ai-models",
		"phase":     dmcrGCPhaseQueued,
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_collector_up", map[string]string{
		"collector": collectorName,
	}, 1)
}

func gatherMetricsAt(t *testing.T, options Options, now time.Time, objects ...client.Object) []*dto.MetricFamily {
	t.Helper()

	scheme := testkit.NewScheme(t, appsv1.AddToScheme, batchv1.AddToScheme)
	reader := testkit.NewFakeClient(t, scheme, nil, objects...)
	registry := prometheus.NewPedanticRegistry()
	collector := NewCollector(reader, reader, slog.New(slog.NewTextHandler(io.Discard, nil)), options)
	collector.now = func() time.Time { return now }
	collector.Register(registry)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	return families
}

type secretListRejectingReader struct {
	client.Reader
}

func (r secretListRejectingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*corev1.SecretList); ok {
		return errUnexpectedCachedSecretList
	}
	return r.Reader.List(ctx, list, opts...)
}

func dmcrGCSecret(name, namespace string, annotations map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      map[string]string{dmcrGCRequestLabelKey: dmcrGCRequestLabelValue},
			Annotations: annotations,
		},
	}
}
