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
	deliverycontract "github.com/deckhouse/ai-models/controller/internal/workloaddelivery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceInjectsManagedSharedDirectVolume(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	legacyRuntimePullSecret := legacyRuntimeImagePullSecretNameForTest(t, owner.GetUID())
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
	template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: legacyRuntimePullSecret}}

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Bindings: []ModelBinding{{
			Name:     "qwen3-14b",
			Artifact: publishedArtifact(),
		}},
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
	if !deliverycontract.VerifyResolvedDeliverySignature(owner.GetNamespace(), template.Annotations, testDeliveryAuthKey) {
		t.Fatalf("expected controller-stamped resolved delivery signature")
	}
	if got := len(template.Spec.ImagePullSecrets); got != 0 {
		t.Fatalf("shared-direct must prune runtime imagePullSecret, got %#v", template.Spec.ImagePullSecrets)
	}
	if len(template.Spec.InitContainers) != 0 {
		t.Fatalf("did not expect per-workload materializer for shared-direct delivery, got %#v", template.Spec.InitContainers)
	}
	if got, want := template.Spec.Containers[0].VolumeMounts[0].MountPath, nodecache.WorkloadNamedModelPath(DefaultCacheMountPath, "qwen3-14b"); got != want {
		t.Fatalf("managed cache mount path = %q, want %q", got, want)
	}
	if len(template.Spec.NodeSelector) != 0 {
		t.Fatalf("shared-direct must not inject node selectors, got %#v", template.Spec.NodeSelector)
	}
	if HasSchedulingGate(template) {
		t.Fatalf("did not expect scheduling gate for ready managed CSI delivery")
	}

	volumeName := managedModelVolumeName(DefaultManagedCacheName, "qwen3-14b")
	volume, found := findVolumeByName(template.Spec.Volumes, volumeName)
	if !found {
		t.Fatalf("expected managed cache volume %q", volumeName)
	}
	if volume.CSI == nil {
		t.Fatalf("expected managed volume to use inline CSI, got %#v", volume.VolumeSource)
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
	secrets := &corev1.SecretList{}
	if err := kubeClient.List(context.Background(), secrets); err != nil {
		t.Fatalf("List(secrets) error = %v", err)
	}
	if len(secrets.Items) != 0 {
		t.Fatalf("shared-direct must not create or delete workload namespace secrets, got %#v", secrets.Items)
	}
}

func TestServicePreservesUserProvidedSharedDirectVolumeAttributes(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
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
	addNodeCacheVolume(template, managedModelVolumeName(DefaultManagedCacheName, "qwen3-14b"))
	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Bindings: []ModelBinding{{
			Name:     "qwen3-14b",
			Artifact: publishedArtifact(),
		}},
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}

	volume, found := findVolumeByName(template.Spec.Volumes, managedModelVolumeName(DefaultManagedCacheName, "qwen3-14b"))
	if !found || volume.CSI == nil {
		t.Fatalf("expected managed CSI volume, got %#v", template.Spec.Volumes)
	}
	if got, want := volume.CSI.VolumeAttributes["user.deckhouse.io/cache"], "enabled"; got != want {
		t.Fatalf("expected user-provided CSI attributes to be preserved, got %q", got)
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
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
		},
		DeliveryAuthKey:         testDeliveryAuthKey,
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithCacheMount("runtime", DefaultManagedCacheName, DefaultCacheMountPath)
	template.Spec.InitContainers = []corev1.Container{{Name: LegacyMaterializerInitContainerName}}
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
		Bindings: []ModelBinding{{
			Name:           "qwen3-14b",
			Artifact:       publishedArtifact(),
			ArtifactFamily: "hf-safetensors-v1",
		}},
		Topology: TopologyHints{ReplicaCount: 1},
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
	volume, found := findVolumeByName(template.Spec.Volumes, managedModelVolumeName(DefaultManagedCacheName, "qwen3-14b"))
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
