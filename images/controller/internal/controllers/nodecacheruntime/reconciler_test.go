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
	"io"
	"log/slog"
	"testing"

	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileCreatesRuntimeResourcesForSelectedNode(t *testing.T) {
	t.Parallel()

	reconciler, kubeClient := newTestReconciler(t,
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-a",
				Labels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"},
			},
		},
	)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "worker-a"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-runtime-worker-a"}, &corev1.Pod{}); err != nil {
		t.Fatalf("expected Pod, got err=%v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-worker-a"}, &corev1.PersistentVolumeClaim{}); err != nil {
		t.Fatalf("expected PVC, got err=%v", err)
	}
	node := &corev1.Node{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: "worker-a"}, node); err != nil {
		t.Fatalf("Get(Node) error = %v", err)
	}
	if got := node.Labels[nodecache.RuntimeReadyNodeLabelKey]; got != "" {
		t.Fatalf("did not expect runtime ready label before Pod/PVC readiness, got %q", got)
	}
}

func TestReconcileLabelsNodeWhenRuntimePodAndPVCAreReady(t *testing.T) {
	t.Parallel()

	reconciler, kubeClient := newTestReconciler(t,
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-a",
				Labels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"},
			},
		},
	)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "worker-a"}}); err != nil {
		t.Fatalf("initial Reconcile() error = %v", err)
	}
	pod := &corev1.Pod{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-runtime-worker-a"}, pod); err != nil {
		t.Fatalf("Get(Pod) error = %v", err)
	}
	pod.Spec.NodeName = "worker-a"
	if err := kubeClient.Update(context.Background(), pod); err != nil {
		t.Fatalf("Update(Pod spec) error = %v", err)
	}
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	if err := kubeClient.Status().Update(context.Background(), pod); err != nil {
		t.Fatalf("Update(Pod status) error = %v", err)
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-worker-a"}, pvc); err != nil {
		t.Fatalf("Get(PVC) error = %v", err)
	}
	pvc.Status.Phase = corev1.ClaimBound
	if err := kubeClient.Status().Update(context.Background(), pvc); err != nil {
		t.Fatalf("Update(PVC status) error = %v", err)
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "worker-a"}}); err != nil {
		t.Fatalf("ready Reconcile() error = %v", err)
	}

	node := &corev1.Node{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: "worker-a"}, node); err != nil {
		t.Fatalf("Get(Node) error = %v", err)
	}
	if got, want := node.Labels[nodecache.RuntimeReadyNodeLabelKey], nodecache.RuntimeReadyNodeLabelValue; got != want {
		t.Fatalf("runtime ready label = %q, want %q", got, want)
	}
}

func TestReconcileDeletesRuntimeResourcesForDeselectedNode(t *testing.T) {
	t.Parallel()

	reconciler, kubeClient := newTestReconciler(t,
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-a",
			Labels: map[string]string{nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue},
		}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ai-models-node-cache-runtime-worker-a", Namespace: "d8-ai-models"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "ai-models-node-cache-worker-a", Namespace: "d8-ai-models"}},
	)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "worker-a"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-runtime-worker-a"}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected Pod deletion, got err=%v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-worker-a"}, &corev1.PersistentVolumeClaim{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected PVC deletion, got err=%v", err)
	}
	node := &corev1.Node{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: "worker-a"}, node); err != nil {
		t.Fatalf("Get(Node) error = %v", err)
	}
	if got := node.Labels[nodecache.RuntimeReadyNodeLabelKey]; got != "" {
		t.Fatalf("expected runtime ready label to be removed, got %q", got)
	}
}

func TestReconcileDeletesRuntimeResourcesForRemovedNode(t *testing.T) {
	t.Parallel()

	reconciler, kubeClient := newTestReconciler(t,
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ai-models-node-cache-runtime-worker-a", Namespace: "d8-ai-models"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "ai-models-node-cache-worker-a", Namespace: "d8-ai-models"}},
	)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "worker-a"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-runtime-worker-a"}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected Pod deletion, got err=%v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-worker-a"}, &corev1.PersistentVolumeClaim{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected PVC deletion, got err=%v", err)
	}
}

func newTestReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&corev1.Pod{}, &corev1.PersistentVolumeClaim{}).
		WithObjects(objects...).
		Build()
	service, err := k8sadapters.NewService(kubeClient, scheme)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &Reconciler{
		client:  kubeClient,
		service: service,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		options: Options{
			Enabled:                true,
			Namespace:              "d8-ai-models",
			RuntimeImage:           "runtime:latest",
			ServiceAccountName:     "ai-models-node-cache-runtime",
			StorageClassName:       "ai-models-node-cache",
			SharedVolumeSize:       "64Gi",
			MaxTotalSize:           "64Gi",
			MaxUnusedAge:           "24h",
			ScanInterval:           "5m",
			OCIAuthSecretName:      "ai-models-dmcr-auth-read",
			DeliveryAuthSecretName: "ai-models-dmcr-auth",
			NodeSelectorLabels:     map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"},
		},
	}, kubeClient
}
