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

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceAppliesRuntimeDeliveryAcrossNamespaces(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
		testkit.NewOCIRegistryCASecret("d8-ai-models", "ai-models-dmcr-ca"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{
			RuntimeImage: "example.com/ai-models:latest",
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
		RegistrySourceCASecretName:   "ai-models-dmcr-ca",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithCacheMount("runtime", "model-cache", DefaultCacheMountPath)

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact:        publishedArtifact(),
		ArtifactFamily:  "hf-safetensors-v1",
		TargetNamespace: "team-a",
		Topology:        TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}

	if got, want := result.CacheMountPath, DefaultCacheMountPath; got != want {
		t.Fatalf("cache mount path = %q, want %q", got, want)
	}
	if got, want := result.ModelPath, nodecache.WorkloadModelPath(DefaultCacheMountPath); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
	if got, want := result.TopologyKind, CacheTopologyPerPod; got != want {
		t.Fatalf("topology kind = %q, want %q", got, want)
	}
	if len(template.Spec.InitContainers) != 1 {
		t.Fatalf("unexpected init containers %#v", template.Spec.InitContainers)
	}
	if got, want := envByName(template.Spec.Containers[0].Env, ModelPathEnv), nodecache.WorkloadModelPath(DefaultCacheMountPath); got != want {
		t.Fatalf("%s = %q, want %q", ModelPathEnv, got, want)
	}
	if got, want := envByName(template.Spec.Containers[0].Env, ModelDigestEnv), publishedArtifact().Digest; got != want {
		t.Fatalf("%s = %q, want %q", ModelDigestEnv, got, want)
	}
	if got, want := envByName(template.Spec.Containers[0].Env, ModelFamilyEnv), "hf-safetensors-v1"; got != want {
		t.Fatalf("%s = %q, want %q", ModelFamilyEnv, got, want)
	}
	if got, want := template.Annotations[ResolvedDigestAnnotation], publishedArtifact().Digest; got != want {
		t.Fatalf("resolved digest annotation = %q, want %q", got, want)
	}
	if got, want := template.Annotations[ResolvedArtifactURIAnnotation], publishedArtifact().URI; got != want {
		t.Fatalf("resolved artifact URI annotation = %q, want %q", got, want)
	}
	if got, want := template.Annotations[ResolvedDeliveryModeAnnotation], string(DeliveryModePerPodFallback); got != want {
		t.Fatalf("resolved delivery mode annotation = %q, want %q", got, want)
	}
	if got, want := template.Annotations[ResolvedDeliveryReasonAnnotation], string(DeliveryReasonWorkloadCacheVolume); got != want {
		t.Fatalf("resolved delivery reason annotation = %q, want %q", got, want)
	}

	authSecretName := projectedAuthSecretName(t, owner.GetUID())
	projectedAuth := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: authSecretName, Namespace: "team-a"}, projectedAuth); err != nil {
		t.Fatalf("Get(projected auth secret) error = %v", err)
	}
	if got, want := string(projectedAuth.Data["username"]), "ai-models"; got != want {
		t.Fatalf("unexpected projected username %q", got)
	}
}

func TestServiceApplyIsIdempotent(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
		testkit.NewOCIRegistryCASecret("d8-ai-models", "ai-models-dmcr-ca"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{
			RuntimeImage: "example.com/ai-models:latest",
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
		RegistrySourceCASecretName:   "ai-models-dmcr-ca",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithCacheMount("runtime", "model-cache", DefaultCacheMountPath)

	request := ApplyRequest{
		Artifact:        publishedArtifact(),
		ArtifactFamily:  "hf-safetensors-v1",
		TargetNamespace: "team-a",
		Topology:        TopologyHints{ReplicaCount: 1},
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, request, template); err != nil {
		t.Fatalf("first ApplyToPodTemplate() error = %v", err)
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, request, template); err != nil {
		t.Fatalf("second ApplyToPodTemplate() error = %v", err)
	}

	if got := len(template.Spec.InitContainers); got != 1 {
		t.Fatalf("expected single init container, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, "registry-ca"); got != 1 {
		t.Fatalf("expected single CA volume, got %d", got)
	}
}

func TestServiceUsesOwnerNamespaceWhenTargetNamespaceIsEmpty(t *testing.T) {
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
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if got, want := result.RegistryAccess.AuthSecretName, projectedAuthSecretName(t, owner.GetUID()); got != want {
		t.Fatalf("auth secret name = %q, want %q", got, want)
	}
	projectedAuth := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: result.RegistryAccess.AuthSecretName, Namespace: owner.GetNamespace()}, projectedAuth); err != nil {
		t.Fatalf("Get(projected auth secret) error = %v", err)
	}
}

func TestServiceInjectsManagedCacheVolumeWhenWorkloadDoesNotProvideMount(t *testing.T) {
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
		ManagedCache: ManagedCacheOptions{
			Enabled:          true,
			StorageClassName: "ai-models-node-cache",
			VolumeSize:       "32Gi",
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}

	if got, want := result.TopologyKind, CacheTopologyPerPod; got != want {
		t.Fatalf("topology kind = %q, want %q", got, want)
	}
	if got, want := template.Annotations[ResolvedDeliveryModeAnnotation], string(DeliveryModePerPodFallback); got != want {
		t.Fatalf("resolved delivery mode annotation = %q, want %q", got, want)
	}
	if got, want := template.Annotations[ResolvedDeliveryReasonAnnotation], string(DeliveryReasonManagedFallbackVolume); got != want {
		t.Fatalf("resolved delivery reason annotation = %q, want %q", got, want)
	}
	if got, want := template.Spec.Containers[0].VolumeMounts[0].MountPath, DefaultCacheMountPath; got != want {
		t.Fatalf("managed cache mount path = %q, want %q", got, want)
	}

	volume, found := findVolumeByName(template.Spec.Volumes, DefaultManagedCacheName)
	if !found {
		t.Fatalf("expected managed cache volume %q to be injected", DefaultManagedCacheName)
	}
	if volume.Ephemeral == nil || volume.Ephemeral.VolumeClaimTemplate == nil {
		t.Fatalf("expected injected volume to use generic ephemeral PVC, got %#v", volume.VolumeSource)
	}
	claim := volume.Ephemeral.VolumeClaimTemplate.Spec
	if got, want := ptr.Deref(claim.StorageClassName, ""), "ai-models-node-cache"; got != want {
		t.Fatalf("storage class = %q, want %q", got, want)
	}
	if got, want := claim.Resources.Requests.Storage().String(), "32Gi"; got != want {
		t.Fatalf("volume size = %q, want %q", got, want)
	}
}
