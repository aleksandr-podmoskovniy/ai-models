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
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentReconcilerRejectsSharedWorkloadPersistentVolumeClaimBridge(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 2, corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "shared-model-cache"},
	})
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-model-cache",
			Namespace: workload.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, pvc, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))
	createLegacyProjectedAccess(t, kubeClient, workload.Namespace, workload.UID)

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}
	var blocked deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &blocked); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if got, want := blocked.Annotations[DeliveryBlockedReasonAnnotation], deliveryBlockedReasonInvalidSpec; got != want {
		t.Fatalf("blocked reason = %q, want %q", got, want)
	}
	if got := blocked.Annotations[DeliveryBlockedMessageAnnotation]; !strings.Contains(got, "does not support explicit cache persistentVolumeClaim") {
		t.Fatalf("unexpected blocked message %q", got)
	}
	if !modeldelivery.HasSchedulingGate(&blocked.Spec.Template) {
		t.Fatalf("expected scheduling gate for invalid runtime delivery spec")
	}
	assertLegacyProjectedAuthSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
	assertLegacyRuntimeImagePullSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
	if events := drainRecordedEvents(t, reconciler); countRecordedEvents(events, "ModelDeliveryBlocked") != 1 {
		t.Fatalf("events = %#v, want one ModelDeliveryBlocked", events)
	}
}
