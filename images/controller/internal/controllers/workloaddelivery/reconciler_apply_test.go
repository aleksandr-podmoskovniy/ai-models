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
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentReconcilerAppliesRuntimeDelivery(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
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
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedArtifactURIAnnotation]; got != testArtifactURI {
		t.Fatalf("resolved artifact URI annotation = %q, want %q", got, testArtifactURI)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryModeAnnotation]; got != string(modeldelivery.DeliveryModeMaterializeBridge) {
		t.Fatalf("resolved delivery mode annotation = %q", got)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryReasonAnnotation]; got != string(modeldelivery.DeliveryReasonWorkloadCacheVolume) {
		t.Fatalf("resolved delivery reason annotation = %q", got)
	}
	if !hasInitContainer(updated.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("expected init container %q", modeldelivery.DefaultInitContainerName)
	}
	if !hasRuntimeEnv(updated.Spec.Template.Spec.Containers, modeldelivery.ModelPathEnv) {
		t.Fatalf("expected runtime env %q", modeldelivery.ModelPathEnv)
	}
	if got, want := runtimeEnvValue(updated.Spec.Template.Spec.Containers, modeldelivery.ModelPathEnv), modeldelivery.ModelPath(modeldelivery.Options{CacheMountPath: modeldelivery.DefaultCacheMountPath}); got != want {
		t.Fatalf("runtime model path env = %q, want %q", got, want)
	}
	if !hasRuntimeEnv(updated.Spec.Template.Spec.Containers, modeldelivery.ModelDigestEnv) {
		t.Fatalf("expected runtime env %q", modeldelivery.ModelDigestEnv)
	}
	if !hasRuntimeEnv(updated.Spec.Template.Spec.Containers, modeldelivery.ModelFamilyEnv) {
		t.Fatalf("expected runtime env %q", modeldelivery.ModelFamilyEnv)
	}
	if got, want := len(updated.Spec.Template.Spec.ImagePullSecrets), 1; got != want {
		t.Fatalf("image pull secrets count = %d, want %d", got, want)
	}
	assertProjectedAuthSecretExists(t, kubeClient, workload.Namespace, workload.UID)
	assertProjectedRuntimeImagePullSecretExists(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerClearsStaleManagedStateWhileReferencedModelIsPending(t *testing.T) {
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
	projectedAuth := testkit.NewOCIRegistryWriteAuthSecret(workload.Namespace, authSecretName)
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName), projectedAuth)

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var cleaned deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &cleaned); err != nil {
		t.Fatalf("Get(cleaned deployment) error = %v", err)
	}
	if hasInitContainer(cleaned.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect init container %q while model is pending", modeldelivery.DefaultInitContainerName)
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; found {
		t.Fatal("did not expect resolved digest annotation while model is pending")
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedArtifactURIAnnotation]; found {
		t.Fatal("did not expect resolved artifact URI annotation while model is pending")
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryModeAnnotation]; found {
		t.Fatal("did not expect resolved delivery mode annotation while model is pending")
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryReasonAnnotation]; found {
		t.Fatal("did not expect resolved delivery reason annotation while model is pending")
	}
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
	assertProjectedRuntimeImagePullSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerInjectsManagedLocalCacheWhenWorkloadHasNoMount(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 1)
	reconciler, kubeClient := newDeploymentReconcilerWithManagedCache(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if !hasInitContainer(updated.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("expected init container %q", modeldelivery.DefaultInitContainerName)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryModeAnnotation]; got != string(modeldelivery.DeliveryModeMaterializeBridge) {
		t.Fatalf("resolved delivery mode annotation = %q", got)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryReasonAnnotation]; got != string(modeldelivery.DeliveryReasonManagedBridgeVolume) {
		t.Fatalf("resolved delivery reason annotation = %q", got)
	}
	if len(updated.Spec.Template.Spec.Containers[0].VolumeMounts) != 1 {
		t.Fatalf("expected managed local cache mount, got %#v", updated.Spec.Template.Spec.Containers[0].VolumeMounts)
	}
	if got, want := updated.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name, modeldelivery.DefaultManagedCacheName; got != want {
		t.Fatalf("managed cache volume name = %q, want %q", got, want)
	}
}
