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
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryModeAnnotation]; got != string(modeldelivery.DeliveryModeSharedDirect) {
		t.Fatalf("resolved delivery mode annotation = %q", got)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryReasonAnnotation]; got != string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane) {
		t.Fatalf("resolved delivery reason annotation = %q", got)
	}
	if hasInitContainer(updated.Spec.Template.Spec.InitContainers, modeldelivery.LegacyMaterializerInitContainerName) {
		t.Fatalf("did not expect init container %q", modeldelivery.LegacyMaterializerInitContainerName)
	}
	if hasRuntimeEnv(updated.Spec.Template.Spec.Containers, "AI_MODELS_MODEL_PATH") {
		t.Fatalf("did not expect legacy runtime env %q", "AI_MODELS_MODEL_PATH")
	}
	if got, want := runtimeEnvValue(updated.Spec.Template.Spec.Containers, modeldelivery.ModelsDirEnv), nodecache.WorkloadModelsDirPath(modeldelivery.DefaultCacheMountPath); got != want {
		t.Fatalf("runtime models dir env = %q, want %q", got, want)
	}
	if !hasRuntimeEnv(updated.Spec.Template.Spec.Containers, modeldelivery.ModelsEnv) {
		t.Fatalf("expected runtime env %q", modeldelivery.ModelsEnv)
	}
	if modeldelivery.HasSchedulingGate(&updated.Spec.Template) {
		t.Fatalf("did not expect scheduling gate %q after delivery apply", modeldelivery.SchedulingGateName)
	}
	if !controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatalf("expected delivery cleanup finalizer, got %#v", updated.Finalizers)
	}
	if got, want := len(updated.Spec.Template.Spec.ImagePullSecrets), 0; got != want {
		t.Fatalf("image pull secrets count = %d, want %d", got, want)
	}
	events := drainRecordedEvents(t, reconciler)
	if got, want := countRecordedEvents(events, "ModelDeliveryApplied"), 1; got != want {
		t.Fatalf("ModelDeliveryApplied events = %d, want %d, all=%#v", got, want, events)
	}
}

func TestDeploymentReconcilerInjectsSharedDirectCache(t *testing.T) {
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
	if hasInitContainer(updated.Spec.Template.Spec.InitContainers, modeldelivery.LegacyMaterializerInitContainerName) {
		t.Fatalf("did not expect init container %q for shared-direct delivery", modeldelivery.LegacyMaterializerInitContainerName)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryModeAnnotation]; got != string(modeldelivery.DeliveryModeSharedDirect) {
		t.Fatalf("resolved delivery mode annotation = %q", got)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDeliveryReasonAnnotation]; got != string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane) {
		t.Fatalf("resolved delivery reason annotation = %q", got)
	}
	if len(updated.Spec.Template.Spec.Containers[0].VolumeMounts) != 1 {
		t.Fatalf("expected managed local cache mount, got %#v", updated.Spec.Template.Spec.Containers[0].VolumeMounts)
	}
	managedVolumeName := modeldelivery.DefaultManagedCacheName + "-" + model.Name
	if got, want := updated.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name, managedVolumeName; got != want {
		t.Fatalf("managed cache volume name = %q, want %q", got, want)
	}
	if len(updated.Spec.Template.Spec.NodeSelector) != 0 {
		t.Fatalf("did not expect node selector injection, got %#v", updated.Spec.Template.Spec.NodeSelector)
	}
	if modeldelivery.HasSchedulingGate(&updated.Spec.Template) {
		t.Fatalf("did not expect scheduling gate for ready managed CSI delivery")
	}
	var volume corev1.Volume
	found := false
	for _, item := range updated.Spec.Template.Spec.Volumes {
		if item.Name == managedVolumeName {
			volume = item
			found = true
			break
		}
	}
	if !found || volume.CSI == nil {
		t.Fatalf("expected managed shared-direct CSI volume, found=%t volume=%#v", found, volume)
	}
	if got, want := volume.CSI.Driver, modeldelivery.NodeCacheCSIDriverName; got != want {
		t.Fatalf("CSI driver = %q, want %q", got, want)
	}
}

func TestDeploymentReconcilerInjectsSharedDirectWithoutDeclaredCSIVolume(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 1)
	reconciler, kubeClient := newDeploymentReconcilerWithOptions(t, modeldelivery.ServiceOptions{
		ManagedCache: modeldelivery.ManagedCacheOptions{
			Enabled: true,
		},
		DeliveryAuthKey:         "test-delivery-auth-key",
		RegistrySourceNamespace: testRegistryNamespace,
	}, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if modeldelivery.HasSchedulingGate(&updated.Spec.Template) {
		t.Fatalf("did not expect scheduling gate for injected managed CSI delivery")
	}
	if got := countVolumeByName(updated.Spec.Template.Spec.Volumes, modeldelivery.DefaultManagedCacheName+"-"+model.Name); got != 1 {
		t.Fatalf("expected injected managed CSI volume, got %d", got)
	}
}

func TestDeploymentReconcilerSuppressesAppliedEventForStaleReconcileWhenLiveTemplateAlreadyMatches(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	stale := workload.DeepCopy()
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected first reconcile result %#v", result)
	}
	if events := drainRecordedEvents(t, reconciler); countRecordedEvents(events, "ModelDeliveryApplied") != 1 {
		t.Fatalf("first reconcile events = %#v, want one ModelDeliveryApplied", events)
	}

	result = reconcileDeployment(t, reconciler, stale)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected stale reconcile result %#v", result)
	}
	if events := drainRecordedEvents(t, reconciler); len(events) != 0 {
		t.Fatalf("unexpected events after stale reconcile: %#v", events)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; got != testDigest {
		t.Fatalf("resolved digest annotation = %q, want %q", got, testDigest)
	}
}

func TestDeploymentReconcilerRepairsTemplateDriftWithoutAppliedEventWhenDeliveryContractIsUnchanged(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected first reconcile result %#v", result)
	}
	drainRecordedEvents(t, reconciler)

	var drifted deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &drifted); err != nil {
		t.Fatalf("Get(drifted deployment) error = %v", err)
	}
	drifted.Spec.Template.Spec.ImagePullSecrets = nil
	if err := kubeClient.Update(context.Background(), &drifted); err != nil {
		t.Fatalf("Update(drifted deployment) error = %v", err)
	}

	result = reconcileDeployment(t, reconciler, &drifted)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected drift reconcile result %#v", result)
	}
	if events := drainRecordedEvents(t, reconciler); len(events) != 0 {
		t.Fatalf("unexpected events after drift repair: %#v", events)
	}

	var repaired deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &repaired); err != nil {
		t.Fatalf("Get(repaired deployment) error = %v", err)
	}
	if got, want := len(repaired.Spec.Template.Spec.ImagePullSecrets), 0; got != want {
		t.Fatalf("image pull secrets count after repair = %d, want %d", got, want)
	}
}

func TestDeploymentReconcilerRecordsAppliedEventWhenResolvedDigestChanges(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected first reconcile result %#v", result)
	}
	drainRecordedEvents(t, reconciler)

	var published modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &published); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	published.Status.Artifact.Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	published.Status.Artifact.URI = "registry.internal.local/ai-models/catalog/namespaced/team-a/gemma@" + published.Status.Artifact.Digest
	if err := kubeClient.Update(context.Background(), &published); err != nil {
		t.Fatalf("Update(model) error = %v", err)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(updated deployment) error = %v", err)
	}

	result = reconcileDeployment(t, reconciler, &updated)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected second reconcile result %#v", result)
	}
	events := drainRecordedEvents(t, reconciler)
	if got, want := countRecordedEvents(events, "ModelDeliveryApplied"), 1; got != want {
		t.Fatalf("ModelDeliveryApplied events = %d, want %d, all=%#v", got, want, events)
	}

	var changed deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &changed); err != nil {
		t.Fatalf("Get(changed deployment) error = %v", err)
	}
	if got, want := changed.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation], published.Status.Artifact.Digest; got != want {
		t.Fatalf("resolved digest annotation = %q, want %q", got, want)
	}
}
