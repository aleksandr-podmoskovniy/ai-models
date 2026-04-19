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
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deployment = appsv1.Deployment

func TestDeploymentReconcilerRemovesManagedStateWhenAnnotationDisappears(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	if _, err := reconciler.reconcileWorkload(context.Background(), workload); err != nil {
		t.Fatalf("initial reconcileWorkload() error = %v", err)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	updated.Annotations = nil
	if err := kubeClient.Update(context.Background(), &updated); err != nil {
		t.Fatalf("Update(deployment) error = %v", err)
	}

	result := reconcileDeployment(t, reconciler, &updated)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var cleaned deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &cleaned); err != nil {
		t.Fatalf("Get(cleaned deployment) error = %v", err)
	}
	if hasInitContainer(cleaned.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect init container %q after annotation removal", modeldelivery.DefaultInitContainerName)
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; found {
		t.Fatal("did not expect resolved digest annotation after annotation removal")
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedArtifactURIAnnotation]; found {
		t.Fatal("did not expect resolved artifact URI annotation after annotation removal")
	}
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerIgnoresUnmanagedWorkloadWithoutAnnotations(t *testing.T) {
	t.Parallel()

	workload := annotatedDeployment(nil, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var unchanged deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &unchanged); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if hasInitContainer(unchanged.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect init container %q", modeldelivery.DefaultInitContainerName)
	}
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerRemovesInjectedManagedCacheStateWhenAnnotationDisappears(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 1)
	reconciler, kubeClient := newDeploymentReconcilerWithManagedCache(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	if _, err := reconciler.reconcileWorkload(context.Background(), workload); err != nil {
		t.Fatalf("initial reconcileWorkload() error = %v", err)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	updated.Annotations = nil
	if err := kubeClient.Update(context.Background(), &updated); err != nil {
		t.Fatalf("Update(deployment) error = %v", err)
	}

	result := reconcileDeployment(t, reconciler, &updated)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var cleaned deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &cleaned); err != nil {
		t.Fatalf("Get(cleaned deployment) error = %v", err)
	}
	if len(cleaned.Spec.Template.Spec.Containers[0].VolumeMounts) != 0 {
		t.Fatalf("did not expect managed cache mount after cleanup, got %#v", cleaned.Spec.Template.Spec.Containers[0].VolumeMounts)
	}
	for _, volume := range cleaned.Spec.Template.Spec.Volumes {
		if volume.Name == modeldelivery.DefaultManagedCacheName {
			t.Fatalf("did not expect managed cache volume %q after cleanup", modeldelivery.DefaultManagedCacheName)
		}
	}
}
