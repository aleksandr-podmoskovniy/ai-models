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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceInjectsManagedSharedDirectVolumeWhenWorkloadDoesNotProvideMount(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	projectedAuthSecret := projectedAuthSecretName(t, owner.GetUID())
	projectedRuntimePullSecret := projectedRuntimeImagePullSecretName(t, owner.GetUID())
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
		readyNode(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: projectedAuthSecret, Namespace: "team-a"},
			Data:       map[string][]byte{"username": []byte("stale")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: projectedRuntimePullSecret, Namespace: "team-a"},
			Type:       corev1.SecretTypeDockerConfigJson,
			Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte("{}")},
		},
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{
			RuntimeImage: "example.com/ai-models:latest",
		},
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
			NodeSelector: map[string]string{
				"ai.deckhouse.io/node-cache":       "true",
				nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
			},
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
		RuntimeImagePullSecretName:   "ai-models-runtime-pull",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")
	template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: projectedRuntimePullSecret}}

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}

	if got, want := result.TopologyKind, CacheTopologyDirect; got != want {
		t.Fatalf("topology kind = %q, want %q", got, want)
	}
	if got, want := template.Annotations[ResolvedDeliveryModeAnnotation], string(DeliveryModeSharedDirect); got != want {
		t.Fatalf("resolved delivery mode annotation = %q, want %q", got, want)
	}
	if got, want := template.Annotations[ResolvedDeliveryReasonAnnotation], string(DeliveryReasonNodeSharedRuntimePlane); got != want {
		t.Fatalf("resolved delivery reason annotation = %q, want %q", got, want)
	}
	if result.RegistryAccess.AuthSecretName != "" || result.RegistryAccess.CASecretName != "" {
		t.Fatalf("shared-direct must not project registry access, got %#v", result.RegistryAccess)
	}
	if got := len(template.Spec.ImagePullSecrets); got != 0 {
		t.Fatalf("shared-direct must prune runtime imagePullSecret, got %#v", template.Spec.ImagePullSecrets)
	}
	if len(template.Spec.InitContainers) != 0 {
		t.Fatalf("did not expect per-workload materializer for shared-direct delivery, got %#v", template.Spec.InitContainers)
	}
	if got, want := template.Spec.Containers[0].VolumeMounts[0].MountPath, DefaultCacheMountPath; got != want {
		t.Fatalf("managed cache mount path = %q, want %q", got, want)
	}
	if got, want := template.Spec.NodeSelector["ai.deckhouse.io/node-cache"], "true"; got != want {
		t.Fatalf("node selector = %q, want %q", got, want)
	}
	if got, want := template.Spec.NodeSelector[nodecache.RuntimeReadyNodeLabelKey], nodecache.RuntimeReadyNodeLabelValue; got != want {
		t.Fatalf("runtime ready selector = %q, want %q", got, want)
	}
	if HasSchedulingGate(template) {
		t.Fatalf("did not expect scheduling gate when a ready node exists")
	}

	volume, found := findVolumeByName(template.Spec.Volumes, DefaultManagedCacheName)
	if !found {
		t.Fatalf("expected managed cache volume %q to be injected", DefaultManagedCacheName)
	}
	if volume.CSI == nil {
		t.Fatalf("expected injected volume to use inline CSI, got %#v", volume.VolumeSource)
	}
	if got, want := volume.CSI.Driver, NodeCacheCSIDriverName; got != want {
		t.Fatalf("CSI driver = %q, want %q", got, want)
	}
	if volume.CSI.ReadOnly == nil || !*volume.CSI.ReadOnly {
		t.Fatalf("expected read-only CSI volume, got %#v", volume.CSI.ReadOnly)
	}
	if got, want := volume.CSI.VolumeAttributes[nodeCacheCSIAttributeArtifactURI], publishedArtifact().URI; got != want {
		t.Fatalf("artifact URI attribute = %q, want %q", got, want)
	}
	if got, want := volume.CSI.VolumeAttributes[nodeCacheCSIAttributeArtifactDigest], publishedArtifact().Digest; got != want {
		t.Fatalf("artifact digest attribute = %q, want %q", got, want)
	}
	secret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: projectedAuthSecret, Namespace: "team-a"}, secret); !apierrors.IsNotFound(err) {
		t.Fatalf("shared-direct must delete stale projected auth secret, got err %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: projectedRuntimePullSecret, Namespace: "team-a"}, secret); !apierrors.IsNotFound(err) {
		t.Fatalf("shared-direct must delete stale runtime pull secret, got err %v", err)
	}
}

func TestServiceRejectsConflictingManagedNodeCacheSelector(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{RuntimeImage: "example.com/ai-models:latest"},
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
			NodeSelector: map[string]string{
				"ai.deckhouse.io/node-cache":       "true",
				nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
			},
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")
	template.Spec.NodeSelector = map[string]string{"ai.deckhouse.io/node-cache": "false"}

	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err == nil || err.Error() != `runtime delivery managed node-cache selector conflicts on "ai.deckhouse.io/node-cache": workload has "false", node-cache requires "true"` {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestServiceKeepsSchedulingGateWhenManagedSharedDirectHasNoReadyNode(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{RuntimeImage: "example.com/ai-models:latest"},
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
			NodeSelector: map[string]string{
				"ai.deckhouse.io/node-cache":       "true",
				nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
			},
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")
	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if !HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate while no ready node-cache runtime node exists")
	}
}

func TestServiceRefreshesManagedSharedDirectVolumeAttributes(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{RuntimeImage: "example.com/ai-models:latest"},
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
			NodeSelector: map[string]string{
				"ai.deckhouse.io/node-cache":       "true",
				nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
			},
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithCacheMount("runtime", DefaultManagedCacheName, DefaultCacheMountPath)
	template.Spec.InitContainers = []corev1.Container{{Name: DefaultInitContainerName}}
	template.Spec.Volumes[0].VolumeSource = corev1.VolumeSource{
		CSI: &corev1.CSIVolumeSource{
			Driver: NodeCacheCSIDriverName,
			VolumeAttributes: map[string]string{
				nodeCacheCSIAttributeArtifactURI:    "old",
				nodeCacheCSIAttributeArtifactDigest: "sha256:old",
			},
		},
	}

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact:       publishedArtifact(),
		ArtifactFamily: "hf-safetensors-v1",
		Topology:       TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}

	if got, want := result.TopologyKind, CacheTopologyDirect; got != want {
		t.Fatalf("topology kind = %q, want %q", got, want)
	}
	if got := len(template.Spec.InitContainers); got != 0 {
		t.Fatalf("expected stale materializer init container to be removed, got %#v", template.Spec.InitContainers)
	}
	volume, found := findVolumeByName(template.Spec.Volumes, DefaultManagedCacheName)
	if !found || volume.CSI == nil {
		t.Fatalf("expected managed CSI volume, got %#v", template.Spec.Volumes)
	}
	if got, want := volume.CSI.VolumeAttributes[nodeCacheCSIAttributeArtifactURI], publishedArtifact().URI; got != want {
		t.Fatalf("artifact URI attribute = %q, want %q", got, want)
	}
	if got, want := volume.CSI.VolumeAttributes[nodeCacheCSIAttributeArtifactDigest], publishedArtifact().Digest; got != want {
		t.Fatalf("artifact digest attribute = %q, want %q", got, want)
	}
	if got, want := volume.CSI.VolumeAttributes[nodeCacheCSIAttributeArtifactFamily], "hf-safetensors-v1"; got != want {
		t.Fatalf("artifact family attribute = %q, want %q", got, want)
	}
}

func readyNode() *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: "worker-a",
		Labels: map[string]string{
			"ai.deckhouse.io/node-cache":       "true",
			nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
		},
	}}
}
