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
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestModelReconcilerEnqueuesGarbageCollectionRequestAndRemovesFinalizerAfterCompletedJob(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	jobName := cleanupJobName(t, model)
	reconciler, kubeClient := newModelReconciler(t, model, completedJob("d8-ai-models", jobName))

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected result %#v", result)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) error = %v", err)
		}
	} else if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected finalizer to be removed once garbage collection is enqueued")
	}

	var requestSecret corev1.Secret
	key := client.ObjectKey{Namespace: "d8-ai-models", Name: dmcrGCRequestSecretName(model.GetUID())}
	if err := kubeClient.Get(context.Background(), key, &requestSecret); err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if requestSecret.Annotations[dmcrGCRequestedAnnotationKey] == "" {
		t.Fatalf("expected queued request annotation on garbage-collection request secret, got %#v", requestSecret.Annotations)
	}
	if requestSecret.Annotations[dmcrGCSwitchAnnotationKey] != "" {
		t.Fatalf("expected active switch to stay empty on queued request secret, got %#v", requestSecret.Annotations)
	}
}

func TestModelReconcilerRemovesFinalizerWhenQueuedGarbageCollectionRequestAlreadyExists(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	jobName := cleanupJobName(t, model)
	gcSecret := requestedGCSecret("d8-ai-models", model.GetUID())
	reconciler, kubeClient := newModelReconciler(t, model, completedJob("d8-ai-models", jobName), gcSecret)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected result %#v", result)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) error = %v", err)
		}
	} else if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected finalizer to be removed when queued garbage-collection request already exists")
	}
}

func TestModelReconcilerRemovesFinalizerWhenCompletedGarbageCollectionSecretAlreadyExists(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")
	jobName := cleanupJobName(t, model)
	gcSecret := completedGCSecret("d8-ai-models", model.GetUID())
	reconciler, kubeClient := newModelReconciler(t, model, completedJob("d8-ai-models", jobName), gcSecret)

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("Get(model) error = %v", err)
		}
	} else if controllerutil.ContainsFinalizer(&updated, Finalizer) {
		t.Fatal("expected finalizer to be removed after completed cleanup when legacy completed garbage-collection secret already exists")
	}

	var requestSecret corev1.Secret
	key := client.ObjectKey{Namespace: "d8-ai-models", Name: dmcrGCRequestSecretName(model.GetUID())}
	if err := kubeClient.Get(context.Background(), key, &requestSecret); err != nil {
		t.Fatalf("expected legacy garbage-collection request secret to remain untouched, got err=%v", err)
	}
	registrySecretName, err := resourcenames.OCIRegistryAuthSecretName(model.GetUID())
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: registrySecretName, Namespace: "d8-ai-models"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected OCI auth secret to be deleted, got err=%v", err)
	}
}
