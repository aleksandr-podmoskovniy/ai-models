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
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceCreatesSharedPVCAndKeepsGate(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil, owner)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		SharedPVC: SharedPVCOptions{
			StorageClassName: "cephfs-rwx",
		},
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := podTemplateWithoutCacheMount("runtime")
	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Bindings: singleModelBinding(),
		Topology: TopologyHints{ReplicaCount: 2},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}

	if got, want := result.TopologyKind, CacheTopologySharedPVC; got != want {
		t.Fatalf("topology kind = %q, want %q", got, want)
	}
	if got, want := result.DeliveryMode, DeliveryModeSharedPVC; got != want {
		t.Fatalf("delivery mode = %q, want %q", got, want)
	}
	if got, want := result.GateReason, DeliveryGateReasonSharedPVCClaimPending; got != want {
		t.Fatalf("gate reason = %q, want %q", got, want)
	}
	if !HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate while SharedPVC claim is pending")
	}
	if got, want := result.ModelPath, nodecache.WorkloadNamedModelPath(DefaultCacheMountPath, "qwen3-14b"); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
	if got, want := envByName(template.Spec.Containers[0].Env, ModelsDirEnv), nodecache.WorkloadModelsDirPath(DefaultCacheMountPath); got != want {
		t.Fatalf("%s = %q, want %q", ModelsDirEnv, got, want)
	}
	if got := len(template.Spec.InitContainers); got != 0 {
		t.Fatalf("did not expect materializer init containers, got %d", got)
	}
	assertSharedPVCMounted(t, template)
	assertSharedPVCClaimCreated(t, kubeClient, owner.GetNamespace())
}

func TestServiceBlocksWhenSharedPVCStorageClassMissing(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil, owner)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Bindings: singleModelBinding(),
		Topology: TopologyHints{ReplicaCount: 2},
	}, podTemplateWithoutCacheMount("runtime"))
	if err == nil {
		t.Fatalf("ApplyToPodTemplate() error = nil, want blocked error")
	}
	if !IsWorkloadContractError(err) {
		t.Fatalf("ApplyToPodTemplate() error = %T, want WorkloadContractError", err)
	}
	if got, want := WorkloadContractReason(err), string(DeliveryGateReasonSharedPVCStorageClassMissing); got != want {
		t.Fatalf("blocked reason = %q, want %q", got, want)
	}
}

func assertSharedPVCMounted(t *testing.T, template *corev1.PodTemplateSpec) {
	t.Helper()
	if got, want := countVolumeByName(template.Spec.Volumes, DefaultSharedPVCVolumeName), 1; got != want {
		t.Fatalf("shared pvc volumes = %d, want %d", got, want)
	}
	volume := template.Spec.Volumes[0]
	if volume.PersistentVolumeClaim == nil || !volume.PersistentVolumeClaim.ReadOnly {
		t.Fatalf("expected read-only shared pvc volume, got %#v", volume)
	}
	if got, want := template.Spec.Containers[0].VolumeMounts[0].MountPath, nodecache.WorkloadModelsDirPath(DefaultCacheMountPath); got != want {
		t.Fatalf("shared pvc mount path = %q, want %q", got, want)
	}
}

func assertSharedPVCClaimCreated(t *testing.T, kubeClient client.Client, namespace string) {
	t.Helper()
	claims := &corev1.PersistentVolumeClaimList{}
	if err := kubeClient.List(context.Background(), claims, client.InNamespace(namespace)); err != nil {
		t.Fatalf("List(PVCs) error = %v", err)
	}
	if got, want := len(claims.Items), 1; got != want {
		t.Fatalf("PVC count = %d, want %d: %#v", got, want, claims.Items)
	}
	claim := claims.Items[0]
	if claim.Spec.StorageClassName == nil || *claim.Spec.StorageClassName != "cephfs-rwx" {
		t.Fatalf("unexpected storageClassName %#v", claim.Spec.StorageClassName)
	}
	if len(claim.Spec.AccessModes) != 1 || claim.Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Fatalf("unexpected access modes %#v", claim.Spec.AccessModes)
	}
	want := resource.MustParse("1Gi")
	if claim.Spec.Resources.Requests.Storage().Cmp(want) != 0 {
		t.Fatalf("storage request = %s, want %s", claim.Spec.Resources.Requests.Storage(), want.String())
	}
}
