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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentReconcilerBlocksWhenNoDeliveryBackendConfigured(t *testing.T) {
	t.Parallel()

	model := readyModelWithArtifactSize(42)
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 2)
	reconciler, kubeClient := newDeploymentReconcilerWithOptions(t, modeldelivery.ServiceOptions{
		RegistrySourceNamespace: testRegistryNamespace,
	}, model, workload)

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var blocked deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &blocked); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if got, want := blocked.Annotations[DeliveryBlockedReasonAnnotation], string(modeldelivery.DeliveryGateReasonSharedPVCStorageClassMissing); got != want {
		t.Fatalf("blocked reason = %q, want %q", got, want)
	}
	if !modeldelivery.HasSchedulingGate(&blocked.Spec.Template) {
		t.Fatalf("expected scheduling gate while no delivery backend is configured")
	}
	if got := sharedPVCCount(t, kubeClient, workload.Namespace); got != 0 {
		t.Fatalf("SharedPVC count = %d, want 0", got)
	}
}

func TestDeploymentReconcilerDeletesSharedPVCWhenModelBecomesPending(t *testing.T) {
	t.Parallel()

	model := readyModelWithArtifactSize(42)
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 2)
	reconciler, kubeClient := newDeploymentReconcilerWithOptions(t, modeldelivery.ServiceOptions{
		SharedPVC: modeldelivery.SharedPVCOptions{
			StorageClassName: "cephfs-rwx",
		},
		RegistrySourceNamespace: testRegistryNamespace,
	}, model, workload)

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected first reconcile result %#v", result)
	}
	if got := sharedPVCCount(t, kubeClient, workload.Namespace); got != 1 {
		t.Fatalf("SharedPVC count after apply = %d, want 1", got)
	}

	var currentModel modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &currentModel); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	currentModel.Status.Phase = modelsv1alpha1.ModelPhasePending
	currentModel.Status.Artifact = nil
	if err := kubeClient.Update(context.Background(), &currentModel); err != nil {
		t.Fatalf("Update(model) error = %v", err)
	}

	var gated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &gated); err != nil {
		t.Fatalf("Get(gated deployment) error = %v", err)
	}
	result = reconcileDeployment(t, reconciler, &gated)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected second reconcile result %#v", result)
	}
	if got := sharedPVCCount(t, kubeClient, workload.Namespace); got != 0 {
		t.Fatalf("SharedPVC count after pending model = %d, want 0", got)
	}
}

func sharedPVCCount(t *testing.T, kubeClient client.Client, namespace string) int {
	t.Helper()

	claims := &corev1.PersistentVolumeClaimList{}
	if err := kubeClient.List(context.Background(), claims, client.InNamespace(namespace)); err != nil {
		t.Fatalf("List(PVCs) error = %v", err)
	}
	return len(claims.Items)
}
