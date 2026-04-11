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
	deletionapp "github.com/deckhouse/ai-models/controller/internal/application/deletion"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBuildFinalizeDeleteRuntimeSkipsOwnerLookupWhenNotNeeded(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "no-owner-needed",
		},
	}

	runtime, err := buildFinalizeDeleteRuntime(secret, cleanuphandle.Handle{}, deletionapp.FinalizeDeleteDecision{RemoveFinalizer: true})
	if err != nil {
		t.Fatalf("buildFinalizeDeleteRuntime() error = %v", err)
	}
	if runtime.hasOwner {
		t.Fatal("did not expect owner lookup when no create/ensure step is requested")
	}
}

func TestMaybeRemoveDeleteFinalizerUsesObservedHandle(t *testing.T) {
	t.Parallel()

	model := testModel()
	model.Finalizers = []string{Finalizer}
	model.Annotations = map[string]string{
		cleanuphandle.AnnotationKey: "{not-json",
	}

	authSecretName, err := resourcenames.OCIRegistryAuthSecretName(model.GetUID())
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	caSecretName, err := resourcenames.OCIRegistryCASecretName(model.GetUID())
	if err != nil {
		t.Fatalf("OCIRegistryCASecretName() error = %v", err)
	}

	reconciler, kubeClient := newModelReconciler(
		t,
		model,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: "d8-ai-models"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: "d8-ai-models"}},
	)

	result, handled, err := reconciler.maybeRemoveDeleteFinalizer(context.Background(), finalizeDeleteRuntime{
		object: model,
		handle: cleanuphandle.Handle{Kind: cleanuphandle.KindBackendArtifact},
	}, true)
	if err != nil {
		t.Fatalf("maybeRemoveDeleteFinalizer() error = %v", err)
	}
	if !handled {
		t.Fatal("expected finalizer removal to be handled")
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("unexpected result %#v", result)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if contains := len(updated.Finalizers) > 0; contains {
		t.Fatalf("expected finalizers to be removed, got %#v", updated.Finalizers)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: authSecretName}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected auth secret to be deleted, got err=%v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: caSecretName}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected projected CA secret to be deleted, got err=%v", err)
	}
}
