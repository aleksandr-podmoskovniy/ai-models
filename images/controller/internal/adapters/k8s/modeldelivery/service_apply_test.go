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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceAppliesSharedDirectWithoutWorkloadNamespaceSecrets(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil, owner, readyNode())
	service := newSharedDirectApplyService(t, kubeClient, scheme)

	template := podTemplateWithoutCacheMount("runtime")
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
	if got, want := result.DeliveryMode, DeliveryModeSharedDirect; got != want {
		t.Fatalf("delivery mode = %q, want %q", got, want)
	}
	if got, want := result.DeliveryReason, DeliveryReasonNodeSharedRuntimePlane; got != want {
		t.Fatalf("delivery reason = %q, want %q", got, want)
	}
	if got, want := result.ModelPath, nodecache.WorkloadModelPath(DefaultCacheMountPath); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
	if len(template.Spec.InitContainers) != 0 {
		t.Fatalf("did not expect init containers, got %#v", template.Spec.InitContainers)
	}
	if len(template.Spec.ImagePullSecrets) != 0 {
		t.Fatalf("did not expect imagePullSecrets, got %#v", template.Spec.ImagePullSecrets)
	}
	if got, want := envByName(template.Spec.Containers[0].Env, ModelPathEnv), nodecache.WorkloadModelPath(DefaultCacheMountPath); got != want {
		t.Fatalf("%s = %q, want %q", ModelPathEnv, got, want)
	}
	if got, want := template.Annotations[ResolvedDeliveryModeAnnotation], string(DeliveryModeSharedDirect); got != want {
		t.Fatalf("resolved delivery mode annotation = %q, want %q", got, want)
	}

	secrets := &corev1.SecretList{}
	if err := kubeClient.List(context.Background(), secrets, client.InNamespace("team-a")); err != nil {
		t.Fatalf("List(workload namespace secrets) error = %v", err)
	}
	if len(secrets.Items) != 0 {
		t.Fatalf("shared-direct must not create workload namespace secrets, got %#v", secrets.Items)
	}
}

func TestServiceApplyIsIdempotentForSharedDirect(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil, owner, readyNode())
	service := newSharedDirectApplyService(t, kubeClient, scheme)
	template := podTemplateWithoutCacheMount("runtime")
	request := ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}

	if _, err := service.ApplyToPodTemplate(context.Background(), owner, request, template); err != nil {
		t.Fatalf("first ApplyToPodTemplate() error = %v", err)
	}
	if _, err := service.ApplyToPodTemplate(context.Background(), owner, request, template); err != nil {
		t.Fatalf("second ApplyToPodTemplate() error = %v", err)
	}

	if got := len(template.Spec.InitContainers); got != 0 {
		t.Fatalf("expected no init containers, got %d", got)
	}
	if got := countVolumeByName(template.Spec.Volumes, DefaultManagedCacheName); got != 1 {
		t.Fatalf("expected single managed CSI volume, got %d", got)
	}
	if got := len(template.Spec.Containers[0].VolumeMounts); got != 1 {
		t.Fatalf("expected single managed CSI mount, got %d", got)
	}
	if got := len(template.Spec.ImagePullSecrets); got != 0 {
		t.Fatalf("expected no imagePullSecrets, got %d", got)
	}
}

func newSharedDirectApplyService(t *testing.T, kubeClient client.Client, scheme *runtime.Scheme) *Service {
	t.Helper()

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
	return service
}
