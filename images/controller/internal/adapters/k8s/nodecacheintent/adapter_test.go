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

package nodecacheintent

import (
	"context"
	"testing"

	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDesiredConfigMapEncodesIntents(t *testing.T) {
	t.Parallel()

	configMap, err := DesiredConfigMap("d8-ai-models", "worker-a", []intentcontract.ArtifactIntent{{
		ArtifactURI: "oci://example/model-a",
		Digest:      "sha256:a",
	}})
	if err != nil {
		t.Fatalf("DesiredConfigMap() error = %v", err)
	}
	if got := configMap.Data[intentcontract.DataKey]; got == "" {
		t.Fatal("expected encoded intent payload")
	}
}

func TestIntentFromPodReadsManagedAnnotations(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				modeldelivery.ResolvedDigestAnnotation:         "sha256:a",
				modeldelivery.ResolvedArtifactURIAnnotation:   "oci://example/model-a",
				modeldelivery.ResolvedArtifactFamilyAnnotation: "gguf-v1",
			},
		},
	}

	intent, found, err := IntentFromPod(pod)
	if err != nil {
		t.Fatalf("IntentFromPod() error = %v", err)
	}
	if !found {
		t.Fatal("expected managed pod intent to be found")
	}
	if got, want := intent.ArtifactURI, "oci://example/model-a"; got != want {
		t.Fatalf("artifact URI = %q, want %q", got, want)
	}
}

func TestClientLoadsNodeIntents(t *testing.T) {
	t.Parallel()

	configMap, err := DesiredConfigMap("d8-ai-models", "worker-a", []intentcontract.ArtifactIntent{{
		ArtifactURI: "oci://example/model-a",
		Digest:      "sha256:a",
	}})
	if err != nil {
		t.Fatalf("DesiredConfigMap() error = %v", err)
	}
	client, err := NewClient(fake.NewSimpleClientset(configMap), "d8-ai-models")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	intents, err := client.LoadNodeIntents(context.Background(), "worker-a")
	if err != nil {
		t.Fatalf("LoadNodeIntents() error = %v", err)
	}
	if got, want := len(intents), 1; got != want {
		t.Fatalf("intent count = %d, want %d", got, want)
	}
}
