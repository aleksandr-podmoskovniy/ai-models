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
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/domain/modelsource"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type objectKind string

const (
	modelObjectKind        objectKind = "model"
	clusterModelObjectKind objectKind = "clustermodel"
)

type objectMetric struct {
	kind             objectKind
	name             string
	namespace        string
	uid              string
	phase            modelsv1alpha1.ModelPhase
	sourceType       string
	format           string
	task             string
	artifactKind     string
	ready            bool
	artifactSizeByte float64
	conditions       []metav1.Condition
}

func newModelMetric(object *modelsv1alpha1.Model) *objectMetric {
	if object == nil {
		return nil
	}

	return newObjectMetric(
		modelObjectKind,
		object.Name,
		object.Namespace,
		string(object.UID),
		object.Spec,
		object.Status,
	)
}

func newClusterModelMetric(object *modelsv1alpha1.ClusterModel) *objectMetric {
	if object == nil {
		return nil
	}

	return newObjectMetric(
		clusterModelObjectKind,
		object.Name,
		"",
		string(object.UID),
		modelSpecFromClusterSpec(object.Spec),
		object.Status,
	)
}

func modelSpecFromClusterSpec(spec modelsv1alpha1.ClusterModelSpec) modelsv1alpha1.ModelSpec {
	return modelsv1alpha1.ModelSpec{
		Source: modelsv1alpha1.ModelSourceSpec{
			URL:    spec.Source.URL,
			Upload: spec.Source.Upload,
		},
	}
}

func newObjectMetric(
	kind objectKind,
	name string,
	namespace string,
	uid string,
	spec modelsv1alpha1.ModelSpec,
	status modelsv1alpha1.ModelStatus,
) *objectMetric {
	return &objectMetric{
		kind:             kind,
		name:             name,
		namespace:        namespace,
		uid:              uid,
		phase:            effectivePhase(status),
		sourceType:       effectiveSourceType(spec, status),
		format:           effectiveFormat(spec, status),
		task:             effectiveTask(spec, status),
		artifactKind:     trimString(artifactKind(status)),
		ready:            conditionTrue(status.Conditions, modelsv1alpha1.ModelConditionReady),
		artifactSizeByte: artifactSize(status),
		conditions:       status.Conditions,
	}
}

func effectivePhase(status modelsv1alpha1.ModelStatus) modelsv1alpha1.ModelPhase {
	if strings.TrimSpace(string(status.Phase)) == "" {
		return modelsv1alpha1.ModelPhasePending
	}

	return status.Phase
}

func effectiveSourceType(spec modelsv1alpha1.ModelSpec, status modelsv1alpha1.ModelStatus) string {
	if status.Source != nil && strings.TrimSpace(string(status.Source.ResolvedType)) != "" {
		return string(status.Source.ResolvedType)
	}

	sourceType, err := modelsource.DetectType(spec.Source)
	if err != nil {
		return ""
	}

	return string(sourceType)
}

func effectiveFormat(spec modelsv1alpha1.ModelSpec, status modelsv1alpha1.ModelStatus) string {
	_ = spec
	if status.Resolved != nil && strings.TrimSpace(string(status.Resolved.Format)) != "" {
		return string(status.Resolved.Format)
	}

	return ""
}

func effectiveTask(spec modelsv1alpha1.ModelSpec, status modelsv1alpha1.ModelStatus) string {
	_ = spec
	if status.Resolved != nil && strings.TrimSpace(status.Resolved.Task) != "" {
		return status.Resolved.Task
	}

	return ""
}

func artifactKind(status modelsv1alpha1.ModelStatus) string {
	if status.Artifact == nil {
		return ""
	}

	return string(status.Artifact.Kind)
}

func artifactSize(status modelsv1alpha1.ModelStatus) float64 {
	if status.Artifact == nil || status.Artifact.SizeBytes == nil {
		return 0
	}

	return float64(*status.Artifact.SizeBytes)
}

func conditionTrue(conditions []metav1.Condition, conditionType modelsv1alpha1.ModelConditionType) bool {
	condition := apimeta.FindStatusCondition(conditions, string(conditionType))
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func trimString(value string) string {
	return strings.TrimSpace(value)
}
