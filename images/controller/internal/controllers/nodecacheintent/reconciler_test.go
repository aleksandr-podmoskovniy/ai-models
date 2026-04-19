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
	"log/slog"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheintent"
	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcilerCreatesPerNodeIntentConfigMap(t *testing.T) {
	t.Parallel()

	reconciler, kubeClient := newReconciler(t,
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"}},
		managedPod("runtime-a", "worker-a", "oci://example/model-a", "sha256:a"),
		managedPod("runtime-b", "worker-a", "oci://example/model-b", "sha256:b"),
	)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-a"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	configMap := &corev1.ConfigMap{}
	if err := kubeClient.Get(context.Background(), types.NamespacedName{Namespace: "d8-ai-models", Name: "ai-models-node-cache-intent-worker-a"}, configMap); err != nil {
		t.Fatalf("Get(configmap) error = %v", err)
	}
	intents, err := intentcontract.DecodeIntents([]byte(configMap.Data[intentcontract.DataKey]))
	if err != nil {
		t.Fatalf("DecodeIntents() error = %v", err)
	}
	if got, want := len(intents), 2; got != want {
		t.Fatalf("intent count = %d, want %d", got, want)
	}
}

func TestReconcilerDeletesIntentConfigMapWhenNodeHasNoManagedPods(t *testing.T) {
	t.Parallel()

	existing, err := k8sadapters.DesiredConfigMap("d8-ai-models", "worker-a", []intentcontract.ArtifactIntent{{
		ArtifactURI: "oci://example/model-a",
		Digest:      "sha256:a",
	}})
	if err != nil {
		t.Fatalf("DesiredConfigMap() error = %v", err)
	}
	reconciler, kubeClient := newReconciler(t,
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"}},
		existing,
	)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-a"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	configMap := &corev1.ConfigMap{}
	err = kubeClient.Get(context.Background(), types.NamespacedName{Namespace: "d8-ai-models", Name: existing.Name}, configMap)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected configmap deletion, got error %v", err)
	}
}

func newReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t)
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&corev1.Pod{}, podNodeNameIndexField, podNodeNameIndexValue).
		WithObjects(objects...).
		Build()

	return &Reconciler{
		client:  kubeClient,
		service: mustService(t, kubeClient),
		logger:  slog.Default(),
		options: Options{Enabled: true, Namespace: "d8-ai-models"},
	}, kubeClient
}

func managedPod(name, nodeName, artifactURI, digest string) *corev1.Pod {
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
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func mustService(t *testing.T, kubeClient client.Client) *k8sadapters.Service {
	t.Helper()

	service, err := k8sadapters.NewService(kubeClient)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}
