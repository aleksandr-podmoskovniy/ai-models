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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	createLegacyProjectedAccess(t, kubeClient, workload.Namespace, workload.UID)

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
	if hasInitContainer(cleaned.Spec.Template.Spec.InitContainers, modeldelivery.LegacyMaterializerInitContainerName) {
		t.Fatalf("did not expect init container %q after annotation removal", modeldelivery.LegacyMaterializerInitContainerName)
	}
	if hasRuntimeEnv(cleaned.Spec.Template.Spec.Containers, "AI_MODELS_MODEL_PATH") {
		t.Fatalf("did not expect runtime env %q after annotation removal", "AI_MODELS_MODEL_PATH")
	}
	if hasRuntimeEnv(cleaned.Spec.Template.Spec.Containers, "AI_MODELS_MODEL_DIGEST") {
		t.Fatalf("did not expect runtime env %q after annotation removal", "AI_MODELS_MODEL_DIGEST")
	}
	if hasRuntimeEnv(cleaned.Spec.Template.Spec.Containers, "AI_MODELS_MODEL_FAMILY") {
		t.Fatalf("did not expect runtime env %q after annotation removal", "AI_MODELS_MODEL_FAMILY")
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; found {
		t.Fatal("did not expect resolved digest annotation after annotation removal")
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedArtifactURIAnnotation]; found {
		t.Fatal("did not expect resolved artifact URI annotation after annotation removal")
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryModeAnnotation]; found {
		t.Fatal("did not expect resolved delivery mode annotation after annotation removal")
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryReasonAnnotation]; found {
		t.Fatal("did not expect resolved delivery reason annotation after annotation removal")
	}
	if got := len(cleaned.Spec.Template.Spec.ImagePullSecrets); got != 0 {
		t.Fatalf("did not expect imagePullSecrets after annotation removal, got %#v", cleaned.Spec.Template.Spec.ImagePullSecrets)
	}
	if controllerutil.ContainsFinalizer(&cleaned, Finalizer) {
		t.Fatalf("did not expect delivery cleanup finalizer after annotation removal, got %#v", cleaned.Finalizers)
	}
	assertLegacyProjectedAuthSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
	assertLegacyRuntimeImagePullSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerFinalizesAppliedDeliveryOnDelete(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	if _, err := reconciler.reconcileWorkload(context.Background(), workload); err != nil {
		t.Fatalf("initial reconcileWorkload() error = %v", err)
	}

	var applied deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &applied); err != nil {
		t.Fatalf("Get(applied deployment) error = %v", err)
	}
	if !controllerutil.ContainsFinalizer(&applied, Finalizer) {
		t.Fatalf("expected delivery cleanup finalizer before delete, got %#v", applied.Finalizers)
	}
	createLegacyProjectedAccess(t, kubeClient, workload.Namespace, workload.UID)
	now := metav1.Now()
	applied.DeletionTimestamp = &now

	result := reconcileDeployment(t, reconciler, &applied)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var finalized deployment
	err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &finalized)
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("Get(finalized deployment) error = %v", err)
	}
	if err == nil && controllerutil.ContainsFinalizer(&finalized, Finalizer) {
		t.Fatalf("expected delivery cleanup finalizer to be removed, got %#v", finalized.Finalizers)
	}
	assertLegacyProjectedAuthSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
	assertLegacyRuntimeImagePullSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
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
	if hasInitContainer(unchanged.Spec.Template.Spec.InitContainers, modeldelivery.LegacyMaterializerInitContainerName) {
		t.Fatalf("did not expect init container %q", modeldelivery.LegacyMaterializerInitContainerName)
	}
	assertLegacyProjectedAuthSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
	assertLegacyRuntimeImagePullSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerIgnoresModuleNamespaceWorkload(t *testing.T) {
	t.Parallel()

	workload := annotatedDeployment(nil, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	workload.Namespace = testRegistryNamespace
	workload.Spec.Template.Annotations = map[string]string{modeldelivery.ResolvedDigestAnnotation: testDigest}
	reconciler, kubeClient := newDeploymentReconciler(t, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var unchanged deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &unchanged); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if got := unchanged.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; got != testDigest {
		t.Fatalf("expected module namespace delivery annotation to remain untouched, got %q", got)
	}
}

func TestDeploymentReconcilerRemovesInjectedManagedCacheStateWhenAnnotationDisappears(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 1)
	reconciler, kubeClient := newDeploymentReconcilerWithManagedCache(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	if _, err := reconciler.reconcileWorkload(context.Background(), workload); err != nil {
		t.Fatalf("initial reconcileWorkload() error = %v", err)
	}
	createLegacyProjectedAccess(t, kubeClient, workload.Namespace, workload.UID)

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
	if controllerutil.ContainsFinalizer(&cleaned, Finalizer) {
		t.Fatalf("did not expect delivery cleanup finalizer after managed cache cleanup, got %#v", cleaned.Finalizers)
	}
	for _, envName := range []string{
		"AI_MODELS_MODEL_PATH",
		"AI_MODELS_MODEL_DIGEST",
		"AI_MODELS_MODEL_FAMILY",
	} {
		if hasRuntimeEnv(cleaned.Spec.Template.Spec.Containers, envName) {
			t.Fatalf("did not expect runtime env %q after cleanup", envName)
		}
	}
	for _, volume := range cleaned.Spec.Template.Spec.Volumes {
		if volume.Name == modeldelivery.DefaultManagedCacheName {
			t.Fatalf("did not expect managed cache volume %q after cleanup", modeldelivery.DefaultManagedCacheName)
		}
	}
	assertLegacyRuntimeImagePullSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
	assertLegacyProjectedAuthSecretAbsent(t, kubeClient, workload.Namespace, workload.UID)
}
