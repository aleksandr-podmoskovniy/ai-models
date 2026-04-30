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
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
)

func TestServicePrunesStaleMultiModelStateWhenSwitchingToSingleSharedDirect(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		readyNode(),
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
		},
		DeliveryAuthKey:         testDeliveryAuthKey,
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")
	multi := ApplyRequest{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []ModelBinding{
			{Name: "qwen3-14b", Artifact: publishedArtifactWithDigest("sha256:primary"), ArtifactFamily: "family-a"},
			{Name: "bge-m3", Artifact: publishedArtifactWithDigest("sha256:embed"), ArtifactFamily: "family-b"},
		},
		Topology: TopologyHints{ReplicaCount: 1},
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, multi, template); err != nil {
		t.Fatalf("multi ApplyToPodTemplate() error = %v", err)
	}
	template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
		Name: ociregistry.CAVolumeName,
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
			SecretName: "stale-registry-ca",
		}},
	})

	single := ApplyRequest{
		Artifact: publishedArtifact(),
		Bindings: []ModelBinding{
			{Name: "gemma", Artifact: publishedArtifact()},
		},
		Topology: TopologyHints{ReplicaCount: 1},
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, single, template); err != nil {
		t.Fatalf("single ApplyToPodTemplate() error = %v", err)
	}

	container := template.Spec.Containers[0]
	for _, name := range []string{
		NamedModelPathEnv("qwen3-14b"),
		NamedModelDigestEnv("qwen3-14b"),
		NamedModelFamilyEnv("qwen3-14b"),
		NamedModelPathEnv("bge-m3"),
		NamedModelDigestEnv("bge-m3"),
		NamedModelFamilyEnv("bge-m3"),
		legacyModelFamilyEnv,
	} {
		if got := envByName(container.Env, name); got != "" {
			t.Fatalf("expected stale env %s to be removed, got %q", name, got)
		}
	}
	if got, want := envByName(container.Env, ModelsDirEnv), ModelsDirPath(NormalizeOptions(Options{})); got != want {
		t.Fatalf("models dir env = %q, want %q", got, want)
	}
	if got := envByName(container.Env, legacyModelPathEnv); got != "" {
		t.Fatalf("did not expect legacy model path env, got %q", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, managedModelVolumeName(DefaultManagedCacheName, "qwen3-14b")); got != 0 {
		t.Fatalf("expected stale qwen CSI volume to be removed, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, managedModelVolumeName(DefaultManagedCacheName, "bge-m3")); got != 0 {
		t.Fatalf("expected stale embed CSI volume to be removed, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, managedModelVolumeName(DefaultManagedCacheName, "gemma")); got != 1 {
		t.Fatalf("expected single managed CSI volume, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, ociregistry.CAVolumeName); got != 0 {
		t.Fatalf("expected orphan registry CA volume to be removed, got %d", got)
	}
	if _, found := template.Annotations[ResolvedModelsAnnotation]; !found {
		t.Fatalf("expected resolved models annotation to remain for single model")
	}
	if _, found := template.Annotations[ResolvedArtifactFamilyAnnotation]; found {
		t.Fatalf("expected stale artifact family annotation to be removed")
	}
}

func TestServicePrunesRemovedModelFromSharedDirectDelivery(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		readyNode(),
	)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
		},
		DeliveryAuthKey:         testDeliveryAuthKey,
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")
	multi := ApplyRequest{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []ModelBinding{
			{Name: "qwen3-14b", Artifact: publishedArtifactWithDigest("sha256:primary")},
			{Name: "bge-m3", Artifact: publishedArtifactWithDigest("sha256:embed")},
		},
		Topology: TopologyHints{ReplicaCount: 1},
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, multi, template); err != nil {
		t.Fatalf("multi ApplyToPodTemplate() error = %v", err)
	}

	primaryOnly := ApplyRequest{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []ModelBinding{
			{Name: "qwen3-14b", Artifact: publishedArtifactWithDigest("sha256:primary")},
		},
		Topology: TopologyHints{ReplicaCount: 1},
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, primaryOnly, template); err != nil {
		t.Fatalf("primary-only ApplyToPodTemplate() error = %v", err)
	}

	if got := len(template.Spec.InitContainers); got != 0 {
		t.Fatalf("did not expect shared-direct init containers, got %#v", template.Spec.InitContainers)
	}
	for _, name := range []string{
		NamedModelPathEnv("bge-m3"),
		NamedModelDigestEnv("bge-m3"),
		NamedModelFamilyEnv("bge-m3"),
	} {
		if got := envByName(template.Spec.Containers[0].Env, name); got != "" {
			t.Fatalf("expected stale env %s to be removed, got %q", name, got)
		}
	}
	if got := template.Annotations[ResolvedModelsAnnotation]; strings.Contains(got, "bge-m3") {
		t.Fatalf("expected resolved models annotation to drop removed model, got %q", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, managedModelVolumeName(DefaultManagedCacheName, "bge-m3")); got != 0 {
		t.Fatalf("expected removed model CSI volume to be pruned, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, managedModelVolumeName(DefaultManagedCacheName, "qwen3-14b")); got != 1 {
		t.Fatalf("expected qwen model CSI volume to remain, got %d", got)
	}
}

func TestServiceRejectsModelMountPathConflict(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		readyNode(),
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
		},
		DeliveryAuthKey:         testDeliveryAuthKey,
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")
	template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{
		Name:      "foreign",
		MountPath: NamedModelPath(NormalizeOptions(Options{}), "main"),
	}}

	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []ModelBinding{
			{Name: "main", Artifact: publishedArtifactWithDigest("sha256:primary")},
		},
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err == nil || err.Error() != "runtime delivery volume mount path conflicts with existing workload mount" {
		t.Fatalf("unexpected error %v", err)
	}
}
