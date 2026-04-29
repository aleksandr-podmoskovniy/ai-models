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
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRayClusterReconcilerAppliesRayServiceModelRefsToHeadAndWorkers(t *testing.T) {
	t.Parallel()

	model := readyClusterModel()
	rayService := annotatedRayService(map[string]string{
		ModelRefsAnnotation: "model=ClusterModel/" + model.Name,
	})
	rayCluster := ownedRayCluster(rayService, 1)
	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Name = "model-cache-pvc"
	pvc.Namespace = rayCluster.GetNamespace()
	pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}

	reconciler, kubeClient := newRayClusterReconciler(t, model, rayService, rayCluster, pvc, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result, err := reconciler.reconcileRayCluster(context.Background(), rayCluster)
	if err != nil {
		t.Fatalf("reconcileRayCluster() error = %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	updated := newRayClusterObject()
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(rayCluster), updated); err != nil {
		t.Fatalf("Get(raycluster) error = %v", err)
	}
	templates, err := rayClusterPodTemplates(updated)
	if err != nil {
		t.Fatalf("rayClusterPodTemplates() error = %v", err)
	}
	if got, want := len(templates), 2; got != want {
		t.Fatalf("template count = %d, want %d", got, want)
	}
	for _, ref := range templates {
		template := ref.Template
		if !hasInitContainer(template.Spec.InitContainers, "ai-models-materializer-model") {
			t.Fatalf("%s template has no alias materializer", ref.Name)
		}
		if got, want := runtimeEnvValue(template.Spec.Containers, modeldelivery.ModelPathEnv), nodecache.WorkloadModelAliasPath(modeldelivery.DefaultCacheMountPath, "model"); got != want {
			t.Fatalf("%s model path env = %q, want %q", ref.Name, got, want)
		}
		if got := template.Annotations[modeldelivery.ResolvedModelsAnnotation]; got == "" {
			t.Fatalf("%s template has no resolved models annotation", ref.Name)
		}
		if !materializerHasEnv(template.Spec.InitContainers, "ai-models-materializer-model", "AI_MODELS_MATERIALIZE_COORDINATION_MODE", modeldelivery.CoordinationModeShared) {
			t.Fatalf("%s template materializer is not coordinated for shared RayCluster PVC", ref.Name)
		}
		if modeldelivery.HasSchedulingGate(template) {
			t.Fatalf("%s template still has scheduling gate", ref.Name)
		}
	}
	if events := drainRecordedEvents(t, reconciler); countRecordedEvents(events, "ModelDeliveryApplied") != 1 {
		t.Fatalf("events = %#v, want one ModelDeliveryApplied", events)
	}
	assertProjectedAuthSecretExists(t, kubeClient, rayCluster.GetNamespace(), rayCluster.GetUID())
	assertProjectedRuntimeImagePullSecretExists(t, kubeClient, rayCluster.GetNamespace(), rayCluster.GetUID())
}

func TestRayClusterPodTemplatesAggregateReplicaHints(t *testing.T) {
	t.Parallel()

	templates, err := rayClusterPodTemplates(ownedRayCluster(annotatedRayService(nil), 3))
	if err != nil {
		t.Fatalf("rayClusterPodTemplates() error = %v", err)
	}
	if got, want := len(templates), 2; got != want {
		t.Fatalf("template count = %d, want %d", got, want)
	}
	for _, ref := range templates {
		if got, want := ref.Hints.ReplicaCount, int32(4); got != want {
			t.Fatalf("%s replica hint = %d, want %d", ref.Name, got, want)
		}
	}
}

func newRayClusterReconciler(t *testing.T, objects ...client.Object) (*baseReconciler, client.Client) {
	return newWorkloadReconcilerWithOptions(t, func(scheme *runtime.Scheme) error {
		registerRayTypes(scheme)
		return nil
	}, defaultServiceOptions(), objects...)
}

func annotatedRayService(annotations map[string]string) *unstructured.Unstructured {
	object := newRayServiceObject().(*unstructured.Unstructured)
	object.SetName("runtime")
	object.SetNamespace("team-a")
	object.SetUID(types.UID("550e8400-e29b-41d4-a716-446655440555"))
	object.SetAnnotations(annotations)
	return object
}

func ownedRayCluster(owner client.Object, workerReplicas int32) *unstructured.Unstructured {
	object := newRayClusterObject().(*unstructured.Unstructured)
	object.SetName("runtime-raycluster")
	object.SetNamespace(owner.GetNamespace())
	object.SetUID(types.UID("550e8400-e29b-41d4-a716-446655440666"))
	object.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: rayServiceGVK.GroupVersion().String(),
		Kind:       rayServiceGVK.Kind,
		Name:       owner.GetName(),
		UID:        owner.GetUID(),
	}})
	object.Object["spec"] = map[string]any{
		"headGroupSpec": map[string]any{
			"template": rayTemplateObject("head"),
		},
		"workerGroupSpecs": []any{
			map[string]any{
				"groupName": "worker",
				"replicas":  int64(workerReplicas),
				"template":  rayTemplateObject("worker"),
			},
		},
	}
	return object
}

func rayTemplateObject(component string) map[string]any {
	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  component,
				Image: "example.com/ray:dev",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "model-cache",
					MountPath: modeldelivery.DefaultCacheMountPath,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "model-cache",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "model-cache-pvc"},
				},
			}},
		},
	}
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&template)
	if err != nil {
		panic(err)
	}
	return raw
}

func materializerHasEnv(containers []corev1.Container, name, envName, envValue string) bool {
	for _, container := range containers {
		if container.Name != name {
			continue
		}
		for _, env := range container.Env {
			if env.Name == envName && env.Value == envValue {
				return true
			}
		}
	}
	return false
}
