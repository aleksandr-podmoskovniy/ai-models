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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCollectorReportsModelAndClusterModelStateMetrics(t *testing.T) {
	model := testkit.NewModel()
	modelArtifactSize := int64(123)
	model.Status = modelsv1alpha1.ModelStatus{
		Phase: modelsv1alpha1.ModelPhaseReady,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: modelsv1alpha1.ModelSourceTypeHuggingFace,
		},
		Artifact: &modelsv1alpha1.ModelArtifactStatus{
			Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
			SizeBytes: &modelArtifactSize,
		},
		Resolved: &modelsv1alpha1.ModelResolvedStatus{
			Task:      "text-generation",
			Format:    "Safetensors",
			Framework: "transformers",
		},
		Conditions: []metav1.Condition{
			{Type: string(modelsv1alpha1.ModelConditionReady), Status: metav1.ConditionTrue},
			{Type: string(modelsv1alpha1.ModelConditionValidated), Status: metav1.ConditionTrue},
		},
	}

	clusterModel := testkit.NewClusterModel()
	clusterModel.Spec.Source.URL = "https://huggingface.co/deepseek-ai/DeepSeek-R1-GGUF"
	clusterModel.Spec.InputFormat = modelsv1alpha1.ModelInputFormatGGUF
	clusterModel.Status = modelsv1alpha1.ModelStatus{
		Phase: modelsv1alpha1.ModelPhaseFailed,
		Source: &modelsv1alpha1.ResolvedSourceStatus{
			ResolvedType: modelsv1alpha1.ModelSourceTypeHuggingFace,
		},
		Resolved: &modelsv1alpha1.ModelResolvedStatus{
			Task:      "text-generation",
			Format:    "GGUF",
			Framework: "llama.cpp",
		},
		Conditions: []metav1.Condition{
			{Type: string(modelsv1alpha1.ModelConditionReady), Status: metav1.ConditionFalse},
			{Type: string(modelsv1alpha1.ModelConditionValidated), Status: metav1.ConditionFalse},
		},
	}

	families := gatherMetrics(t, model, clusterModel)

	assertGaugeValue(t, families, "d8_ai_models_model_status_phase", map[string]string{
		"name":        model.Name,
		"namespace":   model.Namespace,
		"uid":         string(model.UID),
		"phase":       string(modelsv1alpha1.ModelPhaseReady),
		"source_type": string(modelsv1alpha1.ModelSourceTypeHuggingFace),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_model_status_phase", map[string]string{
		"name":        model.Name,
		"namespace":   model.Namespace,
		"uid":         string(model.UID),
		"phase":       string(modelsv1alpha1.ModelPhasePending),
		"source_type": string(modelsv1alpha1.ModelSourceTypeHuggingFace),
	}, 0)
	assertGaugeValue(t, families, "d8_ai_models_model_ready", map[string]string{
		"name":      model.Name,
		"namespace": model.Namespace,
		"uid":       string(model.UID),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_model_validated", map[string]string{
		"name":      model.Name,
		"namespace": model.Namespace,
		"uid":       string(model.UID),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_model_info", map[string]string{
		"name":                 model.Name,
		"namespace":            model.Namespace,
		"uid":                  string(model.UID),
		"resolved_source_type": string(modelsv1alpha1.ModelSourceTypeHuggingFace),
		"format":               "Safetensors",
		"task":                 "text-generation",
		"framework":            "transformers",
		"artifact_kind":        string(modelsv1alpha1.ModelArtifactLocationKindOCI),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_model_artifact_size_bytes", map[string]string{
		"name":      model.Name,
		"namespace": model.Namespace,
		"uid":       string(model.UID),
	}, float64(modelArtifactSize))

	assertGaugeValue(t, families, "d8_ai_models_clustermodel_status_phase", map[string]string{
		"name":        clusterModel.Name,
		"uid":         string(clusterModel.UID),
		"phase":       string(modelsv1alpha1.ModelPhaseFailed),
		"source_type": string(modelsv1alpha1.ModelSourceTypeHuggingFace),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_clustermodel_ready", map[string]string{
		"name": clusterModel.Name,
		"uid":  string(clusterModel.UID),
	}, 0)
	assertGaugeValue(t, families, "d8_ai_models_clustermodel_validated", map[string]string{
		"name": clusterModel.Name,
		"uid":  string(clusterModel.UID),
	}, 0)
	assertGaugeValue(t, families, "d8_ai_models_clustermodel_info", map[string]string{
		"name":                 clusterModel.Name,
		"uid":                  string(clusterModel.UID),
		"resolved_source_type": string(modelsv1alpha1.ModelSourceTypeHuggingFace),
		"format":               "GGUF",
		"task":                 "text-generation",
		"framework":            "llama.cpp",
		"artifact_kind":        "",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_clustermodel_artifact_size_bytes", map[string]string{
		"name": clusterModel.Name,
		"uid":  string(clusterModel.UID),
	}, 0)
}

func TestCollectorFallsBackToPublicSpecWhenStatusIsIncomplete(t *testing.T) {
	model := testkit.NewModel()

	families := gatherMetrics(t, model)

	assertGaugeValue(t, families, "d8_ai_models_model_status_phase", map[string]string{
		"name":        model.Name,
		"namespace":   model.Namespace,
		"uid":         string(model.UID),
		"phase":       string(modelsv1alpha1.ModelPhasePending),
		"source_type": string(modelsv1alpha1.ModelSourceTypeHuggingFace),
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_model_ready", map[string]string{
		"name":      model.Name,
		"namespace": model.Namespace,
		"uid":       string(model.UID),
	}, 0)
	assertGaugeValue(t, families, "d8_ai_models_model_validated", map[string]string{
		"name":      model.Name,
		"namespace": model.Namespace,
		"uid":       string(model.UID),
	}, 0)
	assertGaugeValue(t, families, "d8_ai_models_model_info", map[string]string{
		"name":                 model.Name,
		"namespace":            model.Namespace,
		"uid":                  string(model.UID),
		"resolved_source_type": string(modelsv1alpha1.ModelSourceTypeHuggingFace),
		"format":               string(model.Spec.InputFormat),
		"task":                 model.Spec.RuntimeHints.Task,
		"framework":            "",
		"artifact_kind":        "",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_model_artifact_size_bytes", map[string]string{
		"name":      model.Name,
		"namespace": model.Namespace,
		"uid":       string(model.UID),
	}, 0)
}

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
