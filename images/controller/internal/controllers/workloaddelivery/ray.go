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

package workloaddelivery

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	rayServiceGVK     = schema.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayService"}
	rayServiceListGVK = schema.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayServiceList"}
	rayClusterGVK     = schema.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayCluster"}
	rayClusterListGVK = schema.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayClusterList"}
)

func registerRayTypes(scheme *runtime.Scheme) {
	if scheme == nil {
		return
	}
	scheme.AddKnownTypeWithName(rayServiceGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(rayServiceListGVK, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(rayClusterGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(rayClusterListGVK, &unstructured.UnstructuredList{})
}

func newRayServiceObject() client.Object {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(rayServiceGVK)
	return object
}

func newRayServiceList() client.ObjectList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(rayServiceListGVK)
	return list
}

func newRayClusterObject() client.Object {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(rayClusterGVK)
	return object
}

func newRayClusterList() client.ObjectList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(rayClusterListGVK)
	return list
}

func isRayClusterObject(object client.Object) bool {
	return matchesGVK(object, rayClusterGVK)
}

func matchesGVK(object client.Object, gvk schema.GroupVersionKind) bool {
	if object == nil {
		return false
	}
	actual := object.GetObjectKind().GroupVersionKind()
	return actual.Group == gvk.Group && actual.Version == gvk.Version && actual.Kind == gvk.Kind
}

func rayServiceOwner(object client.Object) (metav1.OwnerReference, bool) {
	for _, ref := range object.GetOwnerReferences() {
		if ref.APIVersion == rayServiceGVK.GroupVersion().String() && ref.Kind == rayServiceGVK.Kind {
			return ref, true
		}
	}
	return metav1.OwnerReference{}, false
}

func rayServiceOwnsCluster(service client.Object, cluster client.Object) bool {
	if service == nil || cluster == nil {
		return false
	}
	ref, found := rayServiceOwner(cluster)
	if !found {
		return false
	}
	if ref.UID != "" {
		return ref.UID == service.GetUID()
	}
	return ref.Name == service.GetName()
}

func rayClusterPodTemplates(object client.Object) ([]workloadPodTemplate, error) {
	rayCluster, ok := object.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("raycluster workload object must be unstructured, got %T", object)
	}

	templates := make([]workloadPodTemplate, 0, 2)
	headTemplate, found, err := nestedPodTemplate(rayCluster, "spec", "headGroupSpec", "template")
	if err != nil {
		return nil, err
	}
	if found {
		templates = append(templates, workloadPodTemplate{
			Name:     "head",
			Template: headTemplate,
			Hints:    modeldelivery.TopologyHints{ReplicaCount: 1},
			Commit: func() error {
				return setNestedPodTemplate(rayCluster, headTemplate, "spec", "headGroupSpec", "template")
			},
		})
	}

	workers, found, err := unstructured.NestedSlice(rayCluster.Object, "spec", "workerGroupSpecs")
	if err != nil {
		return nil, fmt.Errorf("raycluster workerGroupSpecs is invalid: %w", err)
	}
	if found {
		for index := range workers {
			template, ok, err := workerPodTemplate(workers[index])
			if err != nil {
				return nil, fmt.Errorf("raycluster workerGroupSpecs[%d] template is invalid: %w", index, err)
			}
			if !ok {
				continue
			}
			replicas := workerReplicaCount(workers[index])
			workerIndex := index
			templates = append(templates, workloadPodTemplate{
				Name:     "worker-" + strconv.Itoa(index),
				Template: template,
				Hints:    modeldelivery.TopologyHints{ReplicaCount: replicas},
				Commit: func() error {
					return setWorkerPodTemplate(rayCluster, workerIndex, template)
				},
			})
		}
	}

	if len(templates) == 0 {
		return nil, fmt.Errorf("raycluster %s/%s has no supported pod templates", rayCluster.GetNamespace(), rayCluster.GetName())
	}
	applyAggregateReplicaHints(templates)
	return templates, nil
}

func applyAggregateReplicaHints(templates []workloadPodTemplate) {
	total := int64(0)
	for _, ref := range templates {
		replicas := ref.Hints.ReplicaCount
		if replicas <= 0 {
			replicas = 1
		}
		total += int64(replicas)
	}
	if total <= 1 {
		return
	}
	aggregate := positiveInt32(total)
	if aggregate == 0 {
		return
	}
	for index := range templates {
		templates[index].Hints.ReplicaCount = aggregate
	}
}

func positiveInt32(value int64) int32 {
	if value <= 0 || value > math.MaxInt32 {
		return 0
	}
	return int32(value)
}

func nestedPodTemplate(object *unstructured.Unstructured, fields ...string) (*corev1.PodTemplateSpec, bool, error) {
	raw, found, err := unstructured.NestedMap(object.Object, fields...)
	if err != nil || !found {
		return nil, found, err
	}
	template, err := podTemplateFromUnstructured(raw)
	if err != nil {
		return nil, true, err
	}
	return template, true, nil
}

func workerPodTemplate(worker any) (*corev1.PodTemplateSpec, bool, error) {
	workerMap, ok := worker.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("worker group must be an object")
	}
	raw, ok := workerMap["template"].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	template, err := podTemplateFromUnstructured(raw)
	if err != nil {
		return nil, true, err
	}
	return template, true, nil
}

func podTemplateFromUnstructured(raw map[string]any) (*corev1.PodTemplateSpec, error) {
	var template corev1.PodTemplateSpec
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &template); err != nil {
		return nil, err
	}
	return &template, nil
}

func setNestedPodTemplate(object *unstructured.Unstructured, template *corev1.PodTemplateSpec, fields ...string) error {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(template)
	if err != nil {
		return err
	}
	return unstructured.SetNestedMap(object.Object, raw, fields...)
}

func setWorkerPodTemplate(object *unstructured.Unstructured, index int, template *corev1.PodTemplateSpec) error {
	workers, found, err := unstructured.NestedSlice(object.Object, "spec", "workerGroupSpecs")
	if err != nil {
		return err
	}
	if !found || index < 0 || index >= len(workers) {
		return fmt.Errorf("raycluster workerGroupSpecs[%d] is missing", index)
	}
	worker, ok := workers[index].(map[string]any)
	if !ok {
		return fmt.Errorf("raycluster workerGroupSpecs[%d] must be an object", index)
	}
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(template)
	if err != nil {
		return err
	}
	worker["template"] = raw
	workers[index] = worker
	return unstructured.SetNestedSlice(object.Object, workers, "spec", "workerGroupSpecs")
}

func workerReplicaCount(worker any) int32 {
	workerMap, ok := worker.(map[string]any)
	if !ok {
		return 1
	}
	for _, key := range []string{"replicas", "minReplicas"} {
		if replicas := int32FromUnstructured(workerMap[key]); replicas > 0 {
			return replicas
		}
	}
	return 1
}

func int32FromUnstructured(value any) int32 {
	switch typed := value.(type) {
	case int:
		return positiveInt32(int64(typed))
	case int32:
		if typed > 0 {
			return typed
		}
	case int64:
		return positiveInt32(typed)
	case float64:
		if typed > 0 && typed <= math.MaxInt32 && typed == math.Trunc(typed) {
			return int32(typed)
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 32)
		if err == nil {
			return positiveInt32(parsed)
		}
	}
	return 0
}
