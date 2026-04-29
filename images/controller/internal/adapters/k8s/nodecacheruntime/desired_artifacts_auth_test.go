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

package nodecacheruntime

import (
	"context"
	"testing"

	deliverycontract "github.com/deckhouse/ai-models/controller/internal/workloaddelivery"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

const testDeliveryAuthKey = "test-delivery-auth-key"

func TestDesiredArtifactsClientWithAuthKeyIgnoresUnsignedPods(t *testing.T) {
	t.Parallel()

	pod := managedPod("runtime-a", "worker-a", corev1.PodPending, deliverycontract.DeliveryModeSharedDirect, deliverycontract.DeliveryReasonNodeSharedRuntimePlane, "oci://example/model-a", "sha256:a")
	pod.UID = types.UID("pod-a")
	client, err := NewDesiredArtifactsClientWithAuthKey(fake.NewSimpleClientset(pod), testDeliveryAuthKey)
	if err != nil {
		t.Fatalf("NewDesiredArtifactsClientWithAuthKey() error = %v", err)
	}

	artifacts, err := client.LoadNodeDesiredArtifacts(context.Background(), "worker-a")
	if err != nil {
		t.Fatalf("LoadNodeDesiredArtifacts() error = %v", err)
	}
	if len(artifacts) != 0 {
		t.Fatalf("unexpected unsigned artifacts %#v", artifacts)
	}

	allowed, err := client.AllowCSIPublish(context.Background(), "worker-a", map[string]string{
		csiPodNameAttribute:      "runtime-a",
		csiPodNamespaceAttribute: "team-a",
		csiPodUIDAttribute:       "pod-a",
	}, "sha256:a")
	if err != nil {
		t.Fatalf("AllowCSIPublish() error = %v", err)
	}
	if allowed {
		t.Fatal("expected unsigned managed pod to be denied")
	}
}

func TestDesiredArtifactsClientWithAuthKeyAllowsSignedPods(t *testing.T) {
	t.Parallel()

	pod := managedPod("runtime-a", "worker-a", corev1.PodPending, deliverycontract.DeliveryModeSharedDirect, deliverycontract.DeliveryReasonNodeSharedRuntimePlane, "oci://example/model-a", "sha256:a")
	pod.UID = types.UID("pod-a")
	signPodDelivery(pod)
	client, err := NewDesiredArtifactsClientWithAuthKey(fake.NewSimpleClientset(pod), testDeliveryAuthKey)
	if err != nil {
		t.Fatalf("NewDesiredArtifactsClientWithAuthKey() error = %v", err)
	}

	artifacts, err := client.LoadNodeDesiredArtifacts(context.Background(), "worker-a")
	if err != nil {
		t.Fatalf("LoadNodeDesiredArtifacts() error = %v", err)
	}
	if got, want := len(artifacts), 1; got != want {
		t.Fatalf("artifact count = %d, want %d", got, want)
	}

	allowed, err := client.AllowCSIPublish(context.Background(), "worker-a", map[string]string{
		csiPodNameAttribute:      "runtime-a",
		csiPodNamespaceAttribute: "team-a",
		csiPodUIDAttribute:       "pod-a",
	}, "sha256:a")
	if err != nil {
		t.Fatalf("AllowCSIPublish() error = %v", err)
	}
	if !allowed {
		t.Fatal("expected signed managed pod to be allowed")
	}
}

func TestVerifiedDesiredArtifactsFromPodRejectsNamespaceReplay(t *testing.T) {
	t.Parallel()

	pod := managedPod("runtime-a", "worker-a", corev1.PodPending, deliverycontract.DeliveryModeSharedDirect, deliverycontract.DeliveryReasonNodeSharedRuntimePlane, "oci://example/model-a", "sha256:a")
	signPodDelivery(pod)
	pod.Namespace = "other-team"

	artifacts, found, err := VerifiedDesiredArtifactsFromPod(pod, testDeliveryAuthKey)
	if err != nil {
		t.Fatalf("VerifiedDesiredArtifactsFromPod() error = %v", err)
	}
	if found || len(artifacts) != 0 {
		t.Fatalf("unexpected namespace-replayed artifacts %#v found=%v", artifacts, found)
	}
}

func signPodDelivery(pod *corev1.Pod) {
	pod.Annotations[deliverycontract.ResolvedSignatureAnnotation] = deliverycontract.SignResolvedDelivery(pod.Namespace, pod.Annotations, testDeliveryAuthKey)
}
