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

package catalogcleanup

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestModelReconcilerDoesNotAddFinalizerWithoutCleanupHandle(t *testing.T) {
	t.Parallel()

	model := testModel()
	reconciler, kubeClient := newModelReconciler(t, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("did not expect cleanup finalizer without cleanup handle")
	}
}

func TestClusterModelReconcilerAddsFinalizerWhenCleanupHandleExists(t *testing.T) {
	t.Parallel()

	model := testClusterModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/cluster/mixtral-8x7b@sha256:deadbeef")
	reconciler, kubeClient := newClusterModelReconciler(t, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.ClusterModel
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected cleanup finalizer")
	}
}

func TestModelReconcilerRemovesFinalizerOnDeleteWithoutCleanupHandle(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	reconciler, kubeClient := newModelReconciler(t, model)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) error = %v", err)
		}
		return
	}
	if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected stale finalizer to be removed")
	}
}
