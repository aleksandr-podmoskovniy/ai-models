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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentReconcilerBlocksInvalidCacheMountWithoutRetryNoise(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 1)
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected first reconcile result %#v", result)
	}

	var blocked deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &blocked); err != nil {
		t.Fatalf("Get(blocked deployment) error = %v", err)
	}
	if !modeldelivery.HasSchedulingGate(&blocked.Spec.Template) {
		t.Fatalf("expected scheduling gate for invalid runtime delivery spec")
	}
	if got, want := blocked.Spec.Template.Annotations[DeliveryBlockedReasonAnnotation], deliveryBlockedReasonInvalidSpec; got != want {
		t.Fatalf("blocked reason = %q, want %q", got, want)
	}
	if got := blocked.Spec.Template.Annotations[DeliveryBlockedMessageAnnotation]; !strings.Contains(got, "must mount writable model cache") {
		t.Fatalf("blocked message = %q", got)
	}
	if hasInitContainer(blocked.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect runtime delivery init container for invalid workload")
	}
	if events := drainRecordedEvents(t, reconciler); countRecordedEvents(events, "ModelDeliveryBlocked") != 1 {
		t.Fatalf("first reconcile events = %#v, want one ModelDeliveryBlocked", events)
	}
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)

	result = reconcileDeployment(t, reconciler, &blocked)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected second reconcile result %#v", result)
	}
	if events := drainRecordedEvents(t, reconciler); len(events) != 0 {
		t.Fatalf("unexpected repeated events after stable blocked state: %#v", events)
	}
}

func TestDeploymentReconcilerClearsBlockedStateAfterCacheMountAppears(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 1)
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected first reconcile result %#v", result)
	}
	drainRecordedEvents(t, reconciler)

	var fixed deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &fixed); err != nil {
		t.Fatalf("Get(blocked deployment) error = %v", err)
	}
	fixed.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{
		Name:      "model-cache",
		MountPath: modeldelivery.DefaultCacheMountPath,
	}}
	fixed.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "model-cache",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}}
	if err := kubeClient.Update(context.Background(), &fixed); err != nil {
		t.Fatalf("Update(fixed deployment) error = %v", err)
	}

	result = reconcileDeployment(t, reconciler, &fixed)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected second reconcile result %#v", result)
	}

	var applied deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &applied); err != nil {
		t.Fatalf("Get(applied deployment) error = %v", err)
	}
	if got := applied.Spec.Template.Annotations[DeliveryBlockedReasonAnnotation]; got != "" {
		t.Fatalf("blocked reason was not cleared: %q", got)
	}
	if got := applied.Spec.Template.Annotations[DeliveryBlockedMessageAnnotation]; got != "" {
		t.Fatalf("blocked message was not cleared: %q", got)
	}
	if got := applied.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; got != testDigest {
		t.Fatalf("resolved digest annotation = %q, want %q", got, testDigest)
	}
	if events := drainRecordedEvents(t, reconciler); countRecordedEvents(events, "ModelDeliveryApplied") != 1 {
		t.Fatalf("second reconcile events = %#v, want one ModelDeliveryApplied", events)
	}
}

func TestDeploymentReconcilerClearsBlockedStateWhenFixedWorkloadWaitsForModel(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 1)
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected first reconcile result %#v", result)
	}
	drainRecordedEvents(t, reconciler)

	var fixed deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &fixed); err != nil {
		t.Fatalf("Get(blocked deployment) error = %v", err)
	}
	fixed.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{
		Name:      "model-cache",
		MountPath: modeldelivery.DefaultCacheMountPath,
	}}
	fixed.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "model-cache",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}}
	if err := kubeClient.Update(context.Background(), &fixed); err != nil {
		t.Fatalf("Update(fixed deployment) error = %v", err)
	}
	var published modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &published); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	published.Status.Phase = modelsv1alpha1.ModelPhasePending
	published.Status.Artifact = nil
	if err := kubeClient.Update(context.Background(), &published); err != nil {
		t.Fatalf("Update(model) error = %v", err)
	}

	result = reconcileDeployment(t, reconciler, &fixed)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected second reconcile result %#v", result)
	}

	var pending deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &pending); err != nil {
		t.Fatalf("Get(pending deployment) error = %v", err)
	}
	if got := pending.Spec.Template.Annotations[DeliveryBlockedReasonAnnotation]; got != "" {
		t.Fatalf("blocked reason was not cleared: %q", got)
	}
	if got := pending.Spec.Template.Annotations[DeliveryBlockedMessageAnnotation]; got != "" {
		t.Fatalf("blocked message was not cleared: %q", got)
	}
	if !modeldelivery.HasSchedulingGate(&pending.Spec.Template) {
		t.Fatalf("expected scheduling gate while model is pending")
	}
	if events := drainRecordedEvents(t, reconciler); countRecordedEvents(events, "ModelDeliveryPending") != 1 {
		t.Fatalf("second reconcile events = %#v, want one ModelDeliveryPending", events)
	}
}
