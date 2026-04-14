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

package modeldelivery

import (
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceSupportsStatefulSetClaimTemplateTopology(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{
			RuntimeImage: "example.com/ai-models:latest",
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithCacheMount("runtime", "model-cache", DefaultCacheMountPath)

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{
			ReplicaCount: 3,
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "model-cache"},
			}},
		},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if got, want := result.TopologyKind, CacheTopologyPerPod; got != want {
		t.Fatalf("topology kind = %q, want %q", got, want)
	}
}

func TestServiceEnablesSharedCacheCoordinationForSharedRWXPVC(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-model-cache",
			Namespace: owner.GetNamespace(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
		},
	}
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		pvc,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{
			RuntimeImage: "example.com/ai-models:latest",
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithPVCMount("runtime", "model-cache", "shared-model-cache", DefaultCacheMountPath)

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 3},
	}, template)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if got, want := result.TopologyKind, CacheTopologySharedDirect; got != want {
		t.Fatalf("topology kind = %q, want %q", got, want)
	}
	if got := envByName(template.Spec.InitContainers[0].Env, "AI_MODELS_MATERIALIZE_COORDINATION_MODE"); got != CoordinationModeShared {
		t.Fatalf("coordination mode env = %q", got)
	}
	if got := envByName(template.Spec.InitContainers[0].Env, "AI_MODELS_MATERIALIZE_COORDINATION_NAMESPACE"); got != "" {
		t.Fatalf("did not expect coordination namespace env, got %q", got)
	}
}

func TestServiceRejectsSharedDirectPVCWithoutRWX(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-model-cache",
			Namespace: owner.GetNamespace(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		pvc,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{
			RuntimeImage: "example.com/ai-models:latest",
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithPVCMount("runtime", "model-cache", "shared-model-cache", DefaultCacheMountPath)

	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 3},
	}, template)
	if err == nil || err.Error() != "runtime delivery shared persistentVolumeClaim \"shared-model-cache\" for replicas > 1 must support ReadWriteMany" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestServiceRejectsNegativeReplicaCount(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{
			RuntimeImage: "example.com/ai-models:latest",
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithCacheMount("runtime", "model-cache", DefaultCacheMountPath)

	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: -1},
	}, template)
	if err == nil || err.Error() != "runtime delivery replica count must not be negative" {
		t.Fatalf("unexpected error %v", err)
	}
}

func podTemplateWithPVCMount(containerName, volumeName, claimName, mountPath string) *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  containerName,
				Image: "example.com/runtime:latest",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      volumeName,
					MountPath: mountPath,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: claimName,
					},
				},
			}},
		},
	}
}

func envByName(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}
