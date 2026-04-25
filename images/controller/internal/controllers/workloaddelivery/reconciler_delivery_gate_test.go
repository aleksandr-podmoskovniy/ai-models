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
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentReconcilerAppliesClusterModelRuntimeDelivery(t *testing.T) {
	t.Parallel()

	model := readyClusterModel()
	workload := annotatedDeployment(map[string]string{ClusterModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; got != testDigest {
		t.Fatalf("resolved digest annotation = %q, want %q", got, testDigest)
	}
	if modeldelivery.HasSchedulingGate(&updated.Spec.Template) {
		t.Fatalf("did not expect scheduling gate %q after delivery apply", modeldelivery.SchedulingGateName)
	}
}

func TestDeploymentReconcilerKeepsSchedulingGateWhileReferencedModelIsPending(t *testing.T) {
	t.Parallel()

	model := pendingModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	workload.Spec.Template.Annotations = map[string]string{modeldelivery.ResolvedDigestAnnotation: "sha256:old"}
	workload.Spec.Template.Spec.InitContainers = []corev1.Container{{Name: modeldelivery.DefaultInitContainerName}}

	authSecretName, err := resourcenames.OCIRegistryAuthSecretName(workload.UID)
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	runtimeImagePullSecretName, err := resourcenames.RuntimeImagePullSecretName(workload.UID)
	if err != nil {
		t.Fatalf("RuntimeImagePullSecretName() error = %v", err)
	}
	workload.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: runtimeImagePullSecretName}}
	projectedAuth := testkit.NewOCIRegistryWriteAuthSecret(workload.Namespace, authSecretName)
	projectedRuntimePull := testkit.NewOCIRegistryWriteAuthSecret(workload.Namespace, runtimeImagePullSecretName)
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName), projectedAuth, projectedRuntimePull)

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var cleaned deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &cleaned); err != nil {
		t.Fatalf("Get(cleaned deployment) error = %v", err)
	}
	assertPendingDeliveryTemplate(t, &cleaned.Spec.Template)
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
	assertProjectedRuntimeImagePullSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerKeepsSchedulingGateWhenReferencedModelIsMissing(t *testing.T) {
	t.Parallel()

	workload := annotatedDeployment(map[string]string{ModelAnnotation: "missing"}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var gated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &gated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if !modeldelivery.HasSchedulingGate(&gated.Spec.Template) {
		t.Fatalf("expected scheduling gate %q while model is missing", modeldelivery.SchedulingGateName)
	}
	if hasInitContainer(gated.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect init container %q while model is missing", modeldelivery.DefaultInitContainerName)
	}
}

func TestDeploymentReconcilerRemovesSchedulingGateWhenRuntimeDeliveryIsReady(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	modeldelivery.EnsureSchedulingGate(&workload.Spec.Template)
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if modeldelivery.HasSchedulingGate(&updated.Spec.Template) {
		t.Fatalf("did not expect scheduling gate %q after ready delivery", modeldelivery.SchedulingGateName)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; got != testDigest {
		t.Fatalf("resolved digest annotation = %q, want %q", got, testDigest)
	}
}

func TestCronJobReconcilerAppliesRuntimeDeliveryWithoutAdmissionGate(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedCronJob(map[string]string{ModelAnnotation: model.Name}, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newCronJobReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileCronJob(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var updated batchv1.CronJob
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(cronjob) error = %v", err)
	}
	template := &updated.Spec.JobTemplate.Spec.Template
	if got := template.Annotations[modeldelivery.ResolvedDigestAnnotation]; got != testDigest {
		t.Fatalf("resolved digest annotation = %q, want %q", got, testDigest)
	}
	if modeldelivery.HasSchedulingGate(template) {
		t.Fatalf("did not expect scheduling gate %q on CronJob fallback", modeldelivery.SchedulingGateName)
	}
}

func assertPendingDeliveryTemplate(t *testing.T, template *corev1.PodTemplateSpec) {
	t.Helper()

	if hasInitContainer(template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect init container %q while model is pending", modeldelivery.DefaultInitContainerName)
	}
	for _, annotation := range []string{
		modeldelivery.ResolvedDigestAnnotation,
		modeldelivery.ResolvedArtifactURIAnnotation,
		modeldelivery.ResolvedDeliveryModeAnnotation,
		modeldelivery.ResolvedDeliveryReasonAnnotation,
	} {
		if _, found := template.Annotations[annotation]; found {
			t.Fatalf("did not expect annotation %q while model is pending", annotation)
		}
	}
	if !modeldelivery.HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate %q while model is pending", modeldelivery.SchedulingGateName)
	}
	if got := len(template.Spec.ImagePullSecrets); got != 0 {
		t.Fatalf("did not expect imagePullSecrets while model is pending, got %#v", template.Spec.ImagePullSecrets)
	}
}
