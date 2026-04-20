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

func TestDesiredArtifactsClientLoadsNodeArtifactsFromActiveScheduledPods(t *testing.T) {
	t.Parallel()

	client, err := NewDesiredArtifactsClient(fake.NewSimpleClientset(
		managedPod("runtime-a", "worker-a", corev1.PodRunning, "oci://example/model-a", "sha256:a"),
		managedPod("runtime-b", "worker-a", corev1.PodPending, "oci://example/model-b", "sha256:b"),
		managedPod("runtime-c", "worker-b", corev1.PodRunning, "oci://example/model-c", "sha256:c"),
		managedPod("runtime-d", "worker-a", corev1.PodFailed, "oci://example/model-d", "sha256:d"),
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

func TestDesiredArtifactsClientRejectsIncompleteManagedAnnotationsOnTargetNode(t *testing.T) {
	t.Parallel()

	client, err := NewDesiredArtifactsClient(fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runtime-a",
			Namespace: "team-a",
			Annotations: map[string]string{
				modeldelivery.ResolvedDigestAnnotation: "sha256:a",
			},
		},
		Spec:   corev1.PodSpec{NodeName: "worker-a"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}))
	if err != nil {
		t.Fatalf("NewDesiredArtifactsClient() error = %v", err)
	}

	if _, err := client.LoadNodeDesiredArtifacts(context.Background(), "worker-a"); err == nil {
		t.Fatal("expected LoadNodeDesiredArtifacts() to reject incomplete managed pod annotations")
	}
}

func managedPod(name, nodeName string, phase corev1.PodPhase, artifactURI, digest string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "team-a",
			Annotations: map[string]string{
				modeldelivery.ResolvedDigestAnnotation:      digest,
				modeldelivery.ResolvedArtifactURIAnnotation: artifactURI,
			},
		},
		Spec:   corev1.PodSpec{NodeName: nodeName},
		Status: corev1.PodStatus{Phase: phase},
	}
}
