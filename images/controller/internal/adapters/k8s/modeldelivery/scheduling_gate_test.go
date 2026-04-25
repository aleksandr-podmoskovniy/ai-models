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

package modeldelivery

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestSchedulingGateHelpers(t *testing.T) {
	t.Parallel()

	template := &corev1.PodTemplateSpec{}
	if HasSchedulingGate(template) {
		t.Fatal("did not expect gate before ensure")
	}
	if !EnsureSchedulingGate(template) {
		t.Fatal("expected ensure to add gate")
	}
	if !HasSchedulingGate(template) {
		t.Fatal("expected gate after ensure")
	}
	if EnsureSchedulingGate(template) {
		t.Fatal("expected duplicate ensure to be no-op")
	}
	if !RemoveSchedulingGate(template) {
		t.Fatal("expected remove to delete gate")
	}
	if HasSchedulingGate(template) {
		t.Fatal("did not expect gate after remove")
	}
	if RemoveSchedulingGate(template) {
		t.Fatal("expected duplicate remove to be no-op")
	}
}

func TestRemoveSchedulingGatePreservesForeignGates(t *testing.T) {
	t.Parallel()

	template := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{SchedulingGates: []corev1.PodSchedulingGate{
			{Name: "example.com/other"},
			{Name: SchedulingGateName},
		}},
	}

	if !RemoveSchedulingGate(template) {
		t.Fatal("expected remove to delete ai-models gate")
	}
	if got, want := len(template.Spec.SchedulingGates), 1; got != want {
		t.Fatalf("scheduling gates = %d, want %d", got, want)
	}
	if got, want := template.Spec.SchedulingGates[0].Name, "example.com/other"; got != want {
		t.Fatalf("foreign gate = %q, want %q", got, want)
	}
}
