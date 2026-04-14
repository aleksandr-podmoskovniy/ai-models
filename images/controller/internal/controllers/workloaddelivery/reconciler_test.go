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
	"log/slog"
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testRegistryNamespace = "d8-ai-models"
	testRegistryAuthName  = "ai-models-dmcr-auth-read"
	testDigest            = "sha256:d3a98df3d0fff2a2249cf61339492f260122b703621d667259e832681f008d55"
	testArtifactURI       = "registry.internal.local/ai-models/catalog/namespaced/team-a/gemma@" + testDigest
)

func TestDeploymentReconcilerAppliesRuntimeDelivery(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result, err := reconciler.reconcileWorkload(context.Background(), workload)
	if err != nil {
		t.Fatalf("reconcileWorkload() error = %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var updated appsv1.Deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; got != testDigest {
		t.Fatalf("resolved digest annotation = %q, want %q", got, testDigest)
	}
	if !hasInitContainer(updated.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("expected init container %q", modeldelivery.DefaultInitContainerName)
	}
	if hasRuntimeEnv(updated.Spec.Template.Spec.Containers, "AI_MODELS_MODEL_PATH") {
		t.Fatal("did not expect runtime env injection")
	}
	assertProjectedAuthSecretExists(t, kubeClient, workload.Namespace, workload.UID)
}

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

	var updated appsv1.Deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	updated.Annotations = nil
	if err := kubeClient.Update(context.Background(), &updated); err != nil {
		t.Fatalf("Update(deployment) error = %v", err)
	}

	if _, err := reconciler.reconcileWorkload(context.Background(), &updated); err != nil {
		t.Fatalf("remove reconcileWorkload() error = %v", err)
	}

	var cleaned appsv1.Deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &cleaned); err != nil {
		t.Fatalf("Get(cleaned deployment) error = %v", err)
	}
	if hasInitContainer(cleaned.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect init container %q after annotation removal", modeldelivery.DefaultInitContainerName)
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; found {
		t.Fatal("did not expect resolved digest annotation after annotation removal")
	}
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
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

	result, err := reconciler.reconcileWorkload(context.Background(), workload)
	if err != nil {
		t.Fatalf("reconcileWorkload() error = %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var cleaned appsv1.Deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &cleaned); err != nil {
		t.Fatalf("Get(cleaned deployment) error = %v", err)
	}
	if hasInitContainer(cleaned.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect init container %q while model is pending", modeldelivery.DefaultInitContainerName)
	}
	if _, found := cleaned.Spec.Template.Annotations[modeldelivery.ResolvedDigestAnnotation]; found {
		t.Fatal("did not expect resolved digest annotation while model is pending")
	}
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerRejectsSharedPersistentVolumeClaimWithoutRWX(t *testing.T) {
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

	_, err := reconciler.reconcileWorkload(context.Background(), workload)
	if err == nil {
		t.Fatal("expected topology validation error for shared non-RWX PVC")
	}
	if !strings.Contains(err.Error(), "ReadWriteMany") {
		t.Fatalf("unexpected topology error %v", err)
	}
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
}

func TestDeploymentReconcilerIgnoresUnmanagedWorkloadWithoutAnnotations(t *testing.T) {
	t.Parallel()

	workload := annotatedDeployment(nil, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result, err := reconciler.reconcileWorkload(context.Background(), workload)
	if err != nil {
		t.Fatalf("reconcileWorkload() error = %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var unchanged appsv1.Deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &unchanged); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if hasInitContainer(unchanged.Spec.Template.Spec.InitContainers, modeldelivery.DefaultInitContainerName) {
		t.Fatalf("did not expect init container %q", modeldelivery.DefaultInitContainerName)
	}
	assertProjectedAuthSecretDeleted(t, kubeClient, workload.Namespace, workload.UID)
}

func newDeploymentReconciler(t *testing.T, objects ...client.Object) (*baseReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t, appsv1.AddToScheme)
	kubeClient := testkit.NewFakeClient(t, scheme, nil, objects...)
	serviceOptions := modeldelivery.ServiceOptions{
		Render: modeldelivery.Options{
			RuntimeImage: "example.com/ai-models/controller-runtime:dev",
		},
		RegistrySourceNamespace:      testRegistryNamespace,
		RegistrySourceAuthSecretName: testRegistryAuthName,
	}
	service, err := modeldelivery.NewService(kubeClient, scheme, serviceOptions)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return &baseReconciler{
		client:   kubeClient,
		delivery: service,
		options: Options{
			Service: serviceOptions,
		},
		logger:   slog.Default(),
		recorder: record.NewFakeRecorder(16),
	}, kubeClient
}

func readyModel() *modelsv1alpha1.Model {
	model := testkit.NewModel()
	model.Name = "gemma"
	model.Status = modelsv1alpha1.ModelStatus{
		Phase: modelsv1alpha1.ModelPhaseReady,
		Artifact: &modelsv1alpha1.ModelArtifactStatus{
			Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:       testArtifactURI,
			Digest:    testDigest,
			MediaType: "application/vnd.cncf.model.manifest.v1+json",
		},
		Resolved: &modelsv1alpha1.ModelResolvedStatus{Family: "gemma"},
	}
	return model
}

func pendingModel() *modelsv1alpha1.Model {
	model := testkit.NewModel()
	model.Name = "gemma"
	model.Status = modelsv1alpha1.ModelStatus{
		Phase: modelsv1alpha1.ModelPhasePending,
	}
	return model
}

func annotatedDeployment(annotations map[string]string, replicas int32, volumeSource corev1.VolumeSource) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "runtime",
			Namespace:   "team-a",
			UID:         types.UID("550e8400-e29b-41d4-a716-446655440999"),
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "runtime"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "runtime"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "runtime",
						Image: "example.com/runtime:dev",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "model-cache",
							MountPath: modeldelivery.DefaultCacheMountPath,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name:         "model-cache",
						VolumeSource: volumeSource,
					}},
				},
			},
		},
	}
}

func hasInitContainer(containers []corev1.Container, name string) bool {
	for _, container := range containers {
		if container.Name == name {
			return true
		}
	}
	return false
}

func hasRuntimeEnv(containers []corev1.Container, name string) bool {
	for _, container := range containers {
		for _, env := range container.Env {
			if env.Name == name {
				return true
			}
		}
	}
	return false
}

func assertProjectedAuthSecretExists(t *testing.T, kubeClient client.Client, namespace string, ownerUID types.UID) {
	t.Helper()

	name, err := resourcenames.OCIRegistryAuthSecretName(ownerUID)
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{}); err != nil {
		t.Fatalf("expected projected auth secret %s/%s, got err=%v", namespace, name, err)
	}
}

func assertProjectedAuthSecretDeleted(t *testing.T, kubeClient client.Client, namespace string, ownerUID types.UID) {
	t.Helper()

	name, err := resourcenames.OCIRegistryAuthSecretName(ownerUID)
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	err = kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected auth secret %s/%s to be deleted, got err=%v", namespace, name, err)
	}
}
