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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
)

func TestCollectorLeavesCalculatedFieldsEmptyWhenStatusIsIncomplete(t *testing.T) {
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
	assertGaugeValue(t, families, "d8_ai_models_model_condition", map[string]string{
		"name":      model.Name,
		"namespace": model.Namespace,
		"uid":       string(model.UID),
		"type":      string(modelsv1alpha1.ModelConditionReady),
		"status":    "Unknown",
		"reason":    "",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_model_info", map[string]string{
		"name":                 model.Name,
		"namespace":            model.Namespace,
		"uid":                  string(model.UID),
		"resolved_source_type": string(modelsv1alpha1.ModelSourceTypeHuggingFace),
		"format":               "",
		"task":                 "",
		"framework":            "",
		"artifact_kind":        "",
	}, 1)
	assertGaugeValue(t, families, "d8_ai_models_model_artifact_size_bytes", map[string]string{
		"name":      model.Name,
		"namespace": model.Namespace,
		"uid":       string(model.UID),
	}, 0)
}
