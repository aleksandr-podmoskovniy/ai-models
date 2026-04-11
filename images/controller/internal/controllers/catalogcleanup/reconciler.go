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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	deletionapp "github.com/deckhouse/ai-models/controller/internal/application/deletion"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var object modelsv1alpha1.Model
	if err := r.client.Get(ctx, req.NamespacedName, &object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcile(ctx, &object)
}

func (r *ClusterModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var object modelsv1alpha1.ClusterModel
	if err := r.client.Get(ctx, req.NamespacedName, &object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcile(ctx, &object)
}

func (r *baseReconciler) reconcile(ctx context.Context, object client.Object) (ctrl.Result, error) {
	if object.GetDeletionTimestamp().IsZero() {
		return r.reconcileActive(ctx, object)
	}

	return r.reconcileDelete(ctx, object)
}

func (r *baseReconciler) reconcileActive(ctx context.Context, object client.Object) (ctrl.Result, error) {
	decision, err := deletionapp.EnsureCleanupFinalizer(observeFinalizerGuard(object))
	if err != nil {
		return ctrl.Result{}, err
	}
	return r.applyEnsureFinalizerDecision(ctx, object, decision)
}

func (r *baseReconciler) reconcileDelete(ctx context.Context, object client.Object) (ctrl.Result, error) {
	flow, err := r.observeFinalizeDeleteFlow(ctx, object)
	if err != nil {
		return ctrl.Result{}, err
	}
	return r.applyFinalizeDeleteFlow(ctx, flow)
}
