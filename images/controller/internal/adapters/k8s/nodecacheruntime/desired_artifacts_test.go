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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDesiredArtifactFromPodReadsManagedAnnotations(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				modeldelivery.ResolvedDeliveryModeAnnotation:   string(modeldelivery.DeliveryModeSharedDirect),
				modeldelivery.ResolvedDeliveryReasonAnnotation: string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane),
				modeldelivery.ResolvedDigestAnnotation:         "sha256:a",
				modeldelivery.ResolvedArtifactURIAnnotation:    "oci://example/model-a",
				modeldelivery.ResolvedArtifactFamilyAnnotation: "gguf-v1",
			},
		},
	}

	artifact, found, err := DesiredArtifactFromPod(pod)
	if err != nil {
		t.Fatalf("DesiredArtifactFromPod() error = %v", err)
	}
	if !found {
		t.Fatal("expected managed pod published artifact to be found")
	}
	if got, want := artifact.ArtifactURI, "oci://example/model-a"; got != want {
		t.Fatalf("artifact URI = %q, want %q", got, want)
	}
	if got, want := artifact.Digest, "sha256:a"; got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
	if got, want := artifact.Family, "gguf-v1"; got != want {
		t.Fatalf("family = %q, want %q", got, want)
	}
}

func TestDesiredArtifactFromPodIgnoresBridgeAndLegacyPods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
	}{
		{
			name: "materialize-bridge",
			annotations: map[string]string{
				modeldelivery.ResolvedDeliveryModeAnnotation: string(modeldelivery.DeliveryModeMaterializeBridge),
				modeldelivery.ResolvedDigestAnnotation:       "sha256:a",
				modeldelivery.ResolvedArtifactURIAnnotation:  "oci://example/model-a",
			},
		},
		{
			name: "shared-pvc-bridge",
			annotations: map[string]string{
				modeldelivery.ResolvedDeliveryModeAnnotation: string(modeldelivery.DeliveryModeSharedPVCBridge),
				modeldelivery.ResolvedDigestAnnotation:       "sha256:a",
				modeldelivery.ResolvedArtifactURIAnnotation:  "oci://example/model-a",
			},
		},
		{
			name: "shared-direct-without-node-runtime-reason",
			annotations: map[string]string{
				modeldelivery.ResolvedDeliveryModeAnnotation: string(modeldelivery.DeliveryModeSharedDirect),
				modeldelivery.ResolvedDigestAnnotation:       "sha256:a",
				modeldelivery.ResolvedArtifactURIAnnotation:  "oci://example/model-a",
			},
		},
		{
			name: "legacy-without-mode",
			annotations: map[string]string{
				modeldelivery.ResolvedDigestAnnotation:      "sha256:a",
				modeldelivery.ResolvedArtifactURIAnnotation: "oci://example/model-a",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			artifact, found, err := DesiredArtifactFromPod(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			})
			if err != nil {
				t.Fatalf("DesiredArtifactFromPod() error = %v", err)
			}
			if found {
				t.Fatalf("unexpected desired artifact %#v", artifact)
			}
		})
	}
}

func TestDesiredArtifactsClientLoadsOnlySharedDirectArtifactsFromActiveScheduledPods(t *testing.T) {
	t.Parallel()

	client, err := NewDesiredArtifactsClient(fake.NewSimpleClientset(
		managedPod("runtime-a", "worker-a", corev1.PodRunning, string(modeldelivery.DeliveryModeSharedDirect), string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane), "oci://example/model-a", "sha256:a"),
		managedPod("runtime-b", "worker-a", corev1.PodPending, string(modeldelivery.DeliveryModeSharedDirect), string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane), "oci://example/model-b", "sha256:b"),
		managedPod("runtime-c", "worker-b", corev1.PodRunning, string(modeldelivery.DeliveryModeSharedDirect), string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane), "oci://example/model-c", "sha256:c"),
		managedPod("runtime-d", "worker-a", corev1.PodFailed, string(modeldelivery.DeliveryModeSharedDirect), string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane), "oci://example/model-d", "sha256:d"),
		managedPod("runtime-bridge", "worker-a", corev1.PodRunning, string(modeldelivery.DeliveryModeMaterializeBridge), "", "oci://example/model-bridge", "sha256:bridge"),
		managedPod("runtime-shared-pvc", "worker-a", corev1.PodRunning, string(modeldelivery.DeliveryModeSharedPVCBridge), "", "oci://example/model-shared-pvc", "sha256:shared-pvc"),
		managedPod("runtime-shared-direct-misconfigured", "worker-a", corev1.PodRunning, string(modeldelivery.DeliveryModeSharedDirect), "", "oci://example/model-shared-direct", "sha256:shared-direct"),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "runtime-e", Namespace: "team-a"},
			Spec:       corev1.PodSpec{NodeName: "worker-a"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	))
	if err != nil {
		t.Fatalf("NewDesiredArtifactsClient() error = %v", err)
	}

	artifacts, err := client.LoadNodeDesiredArtifacts(context.Background(), "worker-a")
	if err != nil {
		t.Fatalf("LoadNodeDesiredArtifacts() error = %v", err)
	}
	if got, want := len(artifacts), 2; got != want {
		t.Fatalf("artifact count = %d, want %d", got, want)
	}
	if got, want := artifacts[0].Digest, "sha256:a"; got != want {
		t.Fatalf("first digest = %q, want %q", got, want)
	}
	if got, want := artifacts[1].Digest, "sha256:b"; got != want {
		t.Fatalf("second digest = %q, want %q", got, want)
	}
}

func TestDesiredArtifactsClientRejectsIncompleteTrueSharedDirectAnnotationsOnTargetNode(t *testing.T) {
	t.Parallel()

	client, err := NewDesiredArtifactsClient(fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runtime-a",
			Namespace: "team-a",
			Annotations: map[string]string{
				modeldelivery.ResolvedDeliveryModeAnnotation:   string(modeldelivery.DeliveryModeSharedDirect),
				modeldelivery.ResolvedDeliveryReasonAnnotation: string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane),
				modeldelivery.ResolvedDigestAnnotation:         "sha256:a",
			},
		},
		Spec:   corev1.PodSpec{NodeName: "worker-a"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}))
	if err != nil {
		t.Fatalf("NewDesiredArtifactsClient() error = %v", err)
	}

	if _, err := client.LoadNodeDesiredArtifacts(context.Background(), "worker-a"); err == nil {
		t.Fatal("expected LoadNodeDesiredArtifacts() to reject incomplete shared-direct pod annotations")
	}
}

func managedPod(name, nodeName string, phase corev1.PodPhase, deliveryMode, deliveryReason, artifactURI, digest string) *corev1.Pod {
	annotations := map[string]string{
		modeldelivery.ResolvedDeliveryModeAnnotation: deliveryMode,
		modeldelivery.ResolvedDigestAnnotation:       digest,
		modeldelivery.ResolvedArtifactURIAnnotation:  artifactURI,
	}
	if deliveryReason != "" {
		annotations[modeldelivery.ResolvedDeliveryReasonAnnotation] = deliveryReason
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "team-a",
			Annotations: annotations,
		},
		Spec:   corev1.PodSpec{NodeName: nodeName},
		Status: corev1.PodStatus{Phase: phase},
	}
}
