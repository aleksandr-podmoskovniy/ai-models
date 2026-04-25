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

import corev1 "k8s.io/api/core/v1"

const SchedulingGateName = "ai.deckhouse.io/model-delivery"

func EnsureSchedulingGate(template *corev1.PodTemplateSpec) bool {
	if template == nil || HasSchedulingGate(template) {
		return false
	}
	template.Spec.SchedulingGates = append(template.Spec.SchedulingGates, corev1.PodSchedulingGate{
		Name: SchedulingGateName,
	})
	return true
}

func RemoveSchedulingGate(template *corev1.PodTemplateSpec) bool {
	if template == nil || len(template.Spec.SchedulingGates) == 0 {
		return false
	}

	changed := false
	filtered := template.Spec.SchedulingGates[:0]
	for _, gate := range template.Spec.SchedulingGates {
		if gate.Name == SchedulingGateName {
			changed = true
			continue
		}
		filtered = append(filtered, gate)
	}
	if !changed {
		return false
	}
	if len(filtered) == 0 {
		template.Spec.SchedulingGates = nil
		return true
	}
	template.Spec.SchedulingGates = filtered
	return true
}

func HasSchedulingGate(template *corev1.PodTemplateSpec) bool {
	if template == nil {
		return false
	}
	for _, gate := range template.Spec.SchedulingGates {
		if gate.Name == SchedulingGateName {
			return true
		}
	}
	return false
}
