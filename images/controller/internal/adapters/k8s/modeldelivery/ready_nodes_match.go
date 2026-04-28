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
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

func nodeFitsTemplate(node corev1.Node, template *corev1.PodTemplateSpec) bool {
	if template == nil {
		return false
	}
	spec := template.Spec
	if spec.NodeName != "" && spec.NodeName != node.Name {
		return false
	}
	return nodeSelectorMatches(spec.NodeSelector, node.Labels) &&
		nodeAffinityMatches(spec.Affinity, node) &&
		nodeTaintsTolerated(node.Spec.Taints, spec.Tolerations)
}

func nodeSelectorMatches(selector, labels map[string]string) bool {
	for key, want := range selector {
		if labels[key] != want {
			return false
		}
	}
	return true
}

func nodeAffinityMatches(affinity *corev1.Affinity, node corev1.Node) bool {
	if affinity == nil || affinity.NodeAffinity == nil ||
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return true
	}
	for _, term := range affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		if nodeSelectorTermMatches(term, node) {
			return true
		}
	}
	return false
}

func nodeSelectorTermMatches(term corev1.NodeSelectorTerm, node corev1.Node) bool {
	if len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0 {
		return false
	}
	for _, requirement := range term.MatchExpressions {
		value, found := node.Labels[requirement.Key]
		if !nodeSelectorRequirementMatches(requirement, value, found) {
			return false
		}
	}
	for _, requirement := range term.MatchFields {
		value, found := nodeFieldValue(node, requirement.Key)
		if !found || !nodeSelectorRequirementMatches(requirement, value, true) {
			return false
		}
	}
	return true
}

func nodeFieldValue(node corev1.Node, key string) (string, bool) {
	if key == "metadata.name" {
		return node.Name, true
	}
	return "", false
}

func nodeSelectorRequirementMatches(requirement corev1.NodeSelectorRequirement, value string, found bool) bool {
	switch requirement.Operator {
	case corev1.NodeSelectorOpIn:
		return found && stringInSet(value, requirement.Values)
	case corev1.NodeSelectorOpNotIn:
		return !found || !stringInSet(value, requirement.Values)
	case corev1.NodeSelectorOpExists:
		return found
	case corev1.NodeSelectorOpDoesNotExist:
		return !found
	case corev1.NodeSelectorOpGt, corev1.NodeSelectorOpLt:
		return intRequirementMatches(value, requirement)
	default:
		return false
	}
}

func intRequirementMatches(value string, requirement corev1.NodeSelectorRequirement) bool {
	if len(requirement.Values) != 1 {
		return false
	}
	got, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	want, err := strconv.Atoi(requirement.Values[0])
	if err != nil {
		return false
	}
	if requirement.Operator == corev1.NodeSelectorOpGt {
		return got > want
	}
	return got < want
}

func stringInSet(value string, values []string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func nodeTaintsTolerated(taints []corev1.Taint, tolerations []corev1.Toleration) bool {
	for _, taint := range taints {
		if !hardTaint(taint) || taintTolerated(taint, tolerations) {
			continue
		}
		return false
	}
	return true
}

func hardTaint(taint corev1.Taint) bool {
	return taint.Effect == corev1.TaintEffectNoSchedule ||
		taint.Effect == corev1.TaintEffectNoExecute
}

func taintTolerated(taint corev1.Taint, tolerations []corev1.Toleration) bool {
	for _, toleration := range tolerations {
		if tolerationMatchesTaint(toleration, taint) {
			return true
		}
	}
	return false
}

func tolerationMatchesTaint(toleration corev1.Toleration, taint corev1.Taint) bool {
	if toleration.Effect != "" && toleration.Effect != taint.Effect {
		return false
	}
	switch toleration.Operator {
	case corev1.TolerationOpExists:
		return toleration.Key == "" || toleration.Key == taint.Key
	case corev1.TolerationOpEqual, "":
		return toleration.Key == taint.Key && toleration.Value == taint.Value
	default:
		return false
	}
}
