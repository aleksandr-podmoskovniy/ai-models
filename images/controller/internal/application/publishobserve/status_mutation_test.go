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

package publishobserve

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
)

func TestPlanUnsupportedSourceCatalogStatusMutation(t *testing.T) {
	t.Parallel()

	plan, err := PlanUnsupportedSourceCatalogStatusMutation(
		modelsv1alpha1.ModelStatus{},
		7,
		`unsupported source URL host "downloads.example.com"`,
	)
	if err != nil {
		t.Fatalf("PlanUnsupportedSourceCatalogStatusMutation() error = %v", err)
	}
	if got, want := plan.Status.Phase, modelsv1alpha1.ModelPhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if plan.Status.Source != nil {
		t.Fatalf("unexpected source status %#v", plan.Status.Source)
	}
	artifactResolved := apimeta.FindStatusCondition(plan.Status.Conditions, string(modelsv1alpha1.ModelConditionArtifactResolved))
	if artifactResolved == nil || artifactResolved.Reason != string(modelsv1alpha1.ModelConditionReasonUnsupportedSource) {
		t.Fatalf("unexpected artifact resolved condition %#v", artifactResolved)
	}
}
