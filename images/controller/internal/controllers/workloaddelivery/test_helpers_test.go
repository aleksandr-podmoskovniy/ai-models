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
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
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
	testRuntimePullSecret = "ai-models-module-registry"
	testDigest            = "sha256:d3a98df3d0fff2a2249cf61339492f260122b703621d667259e832681f008d55"
	testArtifactURI       = "registry.internal.local/ai-models/catalog/namespaced/team-a/gemma@" + testDigest
)

func newDeploymentReconciler(t *testing.T, objects ...client.Object) (*baseReconciler, client.Client) {
	return newDeploymentReconcilerWithOptions(t, modeldelivery.ServiceOptions{
		Render: modeldelivery.Options{
			RuntimeImage: "example.com/ai-models/controller-runtime:dev",
		},
		RegistrySourceNamespace:      testRegistryNamespace,
		RegistrySourceAuthSecretName: testRegistryAuthName,
		RuntimeImagePullSecretName:   testRuntimePullSecret,
	}, objects...)
}

func newDeploymentReconcilerWithManagedCache(t *testing.T, objects ...client.Object) (*baseReconciler, client.Client) {
	objects = append(objects, readyNodeCacheRuntimeNode())
	return newDeploymentReconcilerWithOptions(t, modeldelivery.ServiceOptions{
		Render: modeldelivery.Options{
			RuntimeImage: "example.com/ai-models/controller-runtime:dev",
		},
		ManagedCache: modeldelivery.ManagedCacheOptions{
			Enabled: true,
			NodeSelector: map[string]string{
				"ai.deckhouse.io/node-cache":       "true",
				nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
			},
		},
		RegistrySourceNamespace:      testRegistryNamespace,
		RegistrySourceAuthSecretName: testRegistryAuthName,
		RuntimeImagePullSecretName:   testRuntimePullSecret,
	}, objects...)
}

func readyNodeCacheRuntimeNode() *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: "worker-a",
		Labels: map[string]string{
			"ai.deckhouse.io/node-cache":       "true",
			nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
		},
	}}
}

func newDeploymentReconcilerWithOptions(t *testing.T, serviceOptions modeldelivery.ServiceOptions, objects ...client.Object) (*baseReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t, appsv1.AddToScheme)
	if serviceOptions.RuntimeImagePullSecretName != "" {
		objects = append(objects, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, serviceOptions.RuntimeImagePullSecretName))
	}
	kubeClient := testkit.NewFakeClient(t, scheme, nil, objects...)
	service, err := modeldelivery.NewService(kubeClient, scheme, serviceOptions)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return &baseReconciler{
		client:   kubeClient,
		reader:   kubeClient,
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

func readyClusterModel() *modelsv1alpha1.ClusterModel {
	model := testkit.NewClusterModel()
	model.Name = "cluster-gemma"
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

func annotatedDeploymentWithoutCacheMount(annotations map[string]string, replicas int32) *appsv1.Deployment {
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

func runtimeEnvValue(containers []corev1.Container, name string) string {
	for _, container := range containers {
		for _, env := range container.Env {
			if env.Name == name {
				return env.Value
			}
		}
	}
	return ""
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

func assertProjectedRuntimeImagePullSecretExists(t *testing.T, kubeClient client.Client, namespace string, ownerUID types.UID) {
	t.Helper()

	name, err := resourcenames.RuntimeImagePullSecretName(ownerUID)
	if err != nil {
		t.Fatalf("RuntimeImagePullSecretName() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{}); err != nil {
		t.Fatalf("expected projected runtime image pull secret %s/%s, got err=%v", namespace, name, err)
	}
}

func assertProjectedRuntimeImagePullSecretDeleted(t *testing.T, kubeClient client.Client, namespace string, ownerUID types.UID) {
	t.Helper()

	name, err := resourcenames.RuntimeImagePullSecretName(ownerUID)
	if err != nil {
		t.Fatalf("RuntimeImagePullSecretName() error = %v", err)
	}
	err = kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected runtime image pull secret %s/%s to be deleted, got err=%v", namespace, name, err)
	}
}

func reconcileDeployment(t *testing.T, reconciler *baseReconciler, workload *appsv1.Deployment) ctrl.Result {
	t.Helper()

	result, err := reconciler.reconcileWorkload(context.Background(), workload)
	if err != nil {
		t.Fatalf("reconcileWorkload() error = %v", err)
	}
	return result
}

func drainRecordedEvents(t *testing.T, reconciler *baseReconciler) []string {
	t.Helper()

	fakeRecorder, ok := reconciler.recorder.(*record.FakeRecorder)
	if !ok {
		t.Fatalf("recorder type = %T, want *record.FakeRecorder", reconciler.recorder)
	}

	var events []string
	for {
		select {
		case event := <-fakeRecorder.Events:
			events = append(events, event)
		default:
			return events
		}
	}
}

func countRecordedEvents(events []string, reason string) int {
	count := 0
	for _, event := range events {
		if strings.Contains(event, reason) {
			count++
		}
	}
	return count
}
