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
		Render: Options{RuntimeImage: "example.com/ai-models:latest"},
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
			NodeSelector: map[string]string{
				"ai.deckhouse.io/node-cache": "true",
			},
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")
	multi := ApplyRequest{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []ModelBinding{
			{Alias: "main", Artifact: publishedArtifactWithDigest("sha256:primary"), ArtifactFamily: "family-a"},
			{Alias: "embed", Artifact: publishedArtifactWithDigest("sha256:embed"), ArtifactFamily: "family-b"},
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
		Topology: TopologyHints{ReplicaCount: 1},
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, single, template); err != nil {
		t.Fatalf("single ApplyToPodTemplate() error = %v", err)
	}

	container := template.Spec.Containers[0]
	for _, name := range []string{
		ModelsDirEnv,
		ModelsEnv,
		NamedModelPathEnv("main"),
		NamedModelDigestEnv("main"),
		NamedModelFamilyEnv("main"),
		NamedModelPathEnv("embed"),
		NamedModelDigestEnv("embed"),
		NamedModelFamilyEnv("embed"),
		ModelFamilyEnv,
	} {
		if got := envByName(container.Env, name); got != "" {
			t.Fatalf("expected stale env %s to be removed, got %q", name, got)
		}
	}
	if got, want := envByName(container.Env, ModelPathEnv), ModelPath(NormalizeOptions(Options{})); got != want {
		t.Fatalf("single model path env = %q, want %q", got, want)
	}
	if got := countVolumeByName(template.Spec.Volumes, DefaultManagedCacheName+"-main"); got != 0 {
		t.Fatalf("expected stale main CSI volume to be removed, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, DefaultManagedCacheName+"-embed"); got != 0 {
		t.Fatalf("expected stale embed CSI volume to be removed, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, DefaultManagedCacheName); got != 1 {
		t.Fatalf("expected single managed CSI volume, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, ociregistry.CAVolumeName); got != 0 {
		t.Fatalf("expected orphan registry CA volume to be removed, got %d", got)
	}
	if _, found := template.Annotations[ResolvedModelsAnnotation]; found {
		t.Fatalf("expected stale resolved models annotation to be removed")
	}
	if _, found := template.Annotations[ResolvedArtifactFamilyAnnotation]; found {
		t.Fatalf("expected stale artifact family annotation to be removed")
	}
}

func TestServicePrunesRemovedAliasFromBridgeDelivery(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render:                       Options{RuntimeImage: "example.com/ai-models:latest"},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithCacheMount("runtime", "model-cache", DefaultCacheMountPath)
	multi := ApplyRequest{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []ModelBinding{
			{Alias: "main", Artifact: publishedArtifactWithDigest("sha256:primary")},
			{Alias: "embed", Artifact: publishedArtifactWithDigest("sha256:embed")},
		},
		Topology: TopologyHints{ReplicaCount: 1},
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, multi, template); err != nil {
		t.Fatalf("multi ApplyToPodTemplate() error = %v", err)
	}

	primaryOnly := ApplyRequest{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []ModelBinding{
			{Alias: "main", Artifact: publishedArtifactWithDigest("sha256:primary")},
		},
		Topology: TopologyHints{ReplicaCount: 1},
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, primaryOnly, template); err != nil {
		t.Fatalf("primary-only ApplyToPodTemplate() error = %v", err)
	}

	if hasContainer(template.Spec.InitContainers, managedInitContainerName(DefaultInitContainerName, "embed")) {
		t.Fatalf("expected removed alias materializer to be pruned")
	}
	for _, name := range []string{
		NamedModelPathEnv("embed"),
		NamedModelDigestEnv("embed"),
		NamedModelFamilyEnv("embed"),
	} {
		if got := envByName(template.Spec.Containers[0].Env, name); got != "" {
			t.Fatalf("expected stale env %s to be removed, got %q", name, got)
		}
	}
	if got := template.Annotations[ResolvedModelsAnnotation]; strings.Contains(got, "embed") {
		t.Fatalf("expected resolved models annotation to drop embed alias, got %q", got)
	}
}

func TestServiceRejectsAliasMountPathConflict(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		readyNode(),
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{RuntimeImage: "example.com/ai-models:latest"},
		ManagedCache: ManagedCacheOptions{
			Enabled:      true,
			NodeSelector: map[string]string{"ai.deckhouse.io/node-cache": "true"},
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
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
			{Alias: "main", Artifact: publishedArtifactWithDigest("sha256:primary")},
		},
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err == nil || err.Error() != "runtime delivery volume mount path conflicts with existing workload mount" {
		t.Fatalf("unexpected error %v", err)
	}
}

func hasContainer(containers []corev1.Container, name string) bool {
	for _, container := range containers {
		if container.Name == name {
			return true
		}
	}
	return false
}
