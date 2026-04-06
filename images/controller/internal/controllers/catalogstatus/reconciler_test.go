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

package catalogstatus

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/controllers/publishrunner"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestModelReconcilerCreatesPublicationOperation(t *testing.T) {
	t.Parallel()

	model := testModel()
	reconciler, kubeClient := newModelReconciler(t, model)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter <= 0 {
		t.Fatalf("expected requeue after operation creation, got %#v", result)
	}

	operationName, err := resourcenames.PublicationOperationConfigMapName(model.UID)
	if err != nil {
		t.Fatalf("PublicationOperationConfigMapName() error = %v", err)
	}

	var operation corev1.ConfigMap
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: operationName, Namespace: "d8-ai-models"}, &operation); err != nil {
		t.Fatalf("expected publication operation configmap to be created: %v", err)
	}

	request, err := publishrunner.RequestFromConfigMap(&operation)
	if err != nil {
		t.Fatalf("RequestFromConfigMap() error = %v", err)
	}
	if got, want := request.Spec.Source.Type, modelsv1alpha1.ModelSourceTypeHuggingFace; got != want {
		t.Fatalf("unexpected source type %q", got)
	}
	if got, want := request.Identity.Reference(), "team-a/deepseek-r1"; got != want {
		t.Fatalf("unexpected publication identity %q", got)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := updated.Status.Phase, modelsv1alpha1.ModelPhasePublishing; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestClusterModelReconcilerCreatesClusterScopedOperationRequest(t *testing.T) {
	t.Parallel()

	clusterModel := testClusterModel()

	reconciler, kubeClient := newClusterModelReconciler(t, clusterModel)
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(clusterModel),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	operationName, err := resourcenames.PublicationOperationConfigMapName(clusterModel.UID)
	if err != nil {
		t.Fatalf("PublicationOperationConfigMapName() error = %v", err)
	}

	var operation corev1.ConfigMap
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: operationName, Namespace: "d8-ai-models"}, &operation); err != nil {
		t.Fatalf("expected publication operation configmap to be created: %v", err)
	}

	request, err := publishrunner.RequestFromConfigMap(&operation)
	if err != nil {
		t.Fatalf("RequestFromConfigMap() error = %v", err)
	}
	if got, want := request.Identity.Scope, publication.ScopeCluster; got != want {
		t.Fatalf("unexpected publication scope %q", got)
	}
	if request.Identity.Namespace != "" {
		t.Fatalf("expected cluster publication namespace to stay empty, got %q", request.Identity.Namespace)
	}
}

func TestModelReconcilerPublishesReadyStatusFromSucceededOperation(t *testing.T) {
	t.Parallel()

	model := testModel()
	operation := succeededOperationForModel(t, model)

	reconciler, kubeClient := newModelReconciler(t, model, operation)
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}

	var annotated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &annotated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if _, found, err := cleanuphandle.FromObject(&annotated); err != nil || !found {
		t.Fatalf("expected cleanup handle annotation after first reconcile, found=%v err=%v", found, err)
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	var ready modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &ready); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := ready.Status.Phase, modelsv1alpha1.ModelPhaseReady; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if ready.Status.Artifact == nil || ready.Status.Artifact.URI != "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef" {
		t.Fatalf("unexpected artifact status %#v", ready.Status.Artifact)
	}
	if ready.Status.Resolved == nil || ready.Status.Resolved.SourceRepoID != "deepseek-ai/DeepSeek-R1" {
		t.Fatalf("unexpected resolved status %#v", ready.Status.Resolved)
	}
}

func TestModelReconcilerMarksFailureFromFailedOperation(t *testing.T) {
	t.Parallel()

	model := testModel()
	operation, err := publishrunner.NewConfigMap("d8-ai-models", publicationports.Request{
		Owner: publicationports.Owner{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      model.Name,
			Namespace: model.Namespace,
			UID:       model.UID,
		},
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: model.Namespace,
			Name:      model.Name,
		},
		Spec: model.Spec,
	})
	if err != nil {
		t.Fatalf("NewConfigMap() error = %v", err)
	}
	if err := publishrunner.SetFailed(operation, "hf import failed"); err != nil {
		t.Fatalf("SetFailed() error = %v", err)
	}

	reconciler, kubeClient := newModelReconciler(t, model, operation)
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var failed modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &failed); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := failed.Status.Phase, modelsv1alpha1.ModelPhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
}

func TestModelReconcilerProjectsWaitForUploadStatus(t *testing.T) {
	t.Parallel()

	model := testUploadModel()
	operation, err := publishrunner.NewConfigMap("d8-ai-models", publicationports.Request{
		Owner: publicationports.Owner{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      model.Name,
			Namespace: model.Namespace,
			UID:       model.UID,
		},
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: model.Namespace,
			Name:      model.Name,
		},
		Spec: model.Spec,
	})
	if err != nil {
		t.Fatalf("NewConfigMap() error = %v", err)
	}
	if err := publishrunner.SetRunning(operation, "ai-model-upload-550e8400-e29b-41d4-a716-44665544"); err != nil {
		t.Fatalf("SetRunning() error = %v", err)
	}
	expiresAt := metav1.Now()
	if err := publishrunner.SetUploadReady(operation, modelsv1alpha1.ModelUploadStatus{
		ExpiresAt:  &expiresAt,
		Repository: "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1-upload/550e8400-e29b-41d4-a716-446655440111:published",
		Command:    "curl -T file",
	}); err != nil {
		t.Fatalf("SetUploadReady() error = %v", err)
	}

	reconciler, kubeClient := newModelReconciler(t, model, operation)
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(model),
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &updated); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := updated.Status.Phase, modelsv1alpha1.ModelPhaseWaitForUpload; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if updated.Status.Upload == nil || updated.Status.Upload.Command != "curl -T file" {
		t.Fatalf("unexpected upload status %#v", updated.Status.Upload)
	}
}
