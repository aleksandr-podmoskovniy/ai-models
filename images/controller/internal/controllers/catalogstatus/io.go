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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishobserve"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func cleanupHandlePresent(object client.Object) (bool, error) {
	_, found, err := cleanuphandle.FromObject(object)
	return found, err
}

func (r *baseReconciler) ensureCleanupHandle(ctx context.Context, object client.Object, handle cleanuphandle.Handle) (bool, error) {
	existing, found, err := cleanuphandle.FromObject(object)
	if err != nil {
		return false, err
	}
	if found && apiequality.Semantic.DeepEqual(existing, handle) {
		return false, nil
	}
	if err := cleanuphandle.SetOnObject(object, handle); err != nil {
		return false, err
	}
	if err := r.client.Update(ctx, object); err != nil {
		return false, err
	}

	return true, nil
}

func (r *baseReconciler) updateStatus(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	desired modelsv1alpha1.ModelStatus,
) error {
	if apiequality.Semantic.DeepEqual(*current, desired) {
		return nil
	}

	if err := modelobject.SetStatus(object, desired); err != nil {
		return err
	}
	return r.client.Status().Update(ctx, object)
}

func (r *baseReconciler) applyMutationPlan(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	plan publicationapp.CatalogStatusMutationPlan,
	deleteFn func(context.Context) error,
) (ctrl.Result, error) {
	if !plan.DeleteRuntime {
		deleteFn = nil
	}
	if plan.DeleteRuntimeBeforePersist && deleteFn != nil {
		if err := deleteFn(ctx); err != nil {
			return ctrl.Result{}, err
		}
		deleteFn = nil
	}
	if plan.CleanupHandle != nil {
		updated, err := r.ensureCleanupHandle(ctx, object, *plan.CleanupHandle)
		if err != nil {
			return ctrl.Result{}, err
		}
		if updated {
			return ctrl.Result{Requeue: true}, nil
		}
	}
	if err := r.updateStatus(ctx, object, current, plan.Status); err != nil {
		return ctrl.Result{}, err
	}
	if deleteFn != nil {
		if err := deleteFn(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}
	if plan.Requeue {
		return ctrl.Result{RequeueAfter: statusPollInterval}, nil
	}
	return ctrl.Result{}, nil
}

func (r *baseReconciler) applyRuntimeObservation(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	decision publicationapp.RuntimeObservationDecision,
	deleteFn func(context.Context) error,
) (ctrl.Result, error) {
	plan, err := publicationapp.PlanCatalogStatusMutation(publicationapp.CatalogStatusMutationInput{
		Current: *current,
		Runtime: publicationapp.CatalogStatusRuntimeResult{
			Generation:    object.GetGeneration(),
			SourceType:    sourceType,
			Observation:   decision.Observation,
			DeleteRuntime: decision.DeleteRuntime,
		},
	})
	if err != nil {
		return r.failPublication(ctx, object, current, sourceType, err.Error())
	}
	return r.applyMutationPlan(ctx, object, current, plan, deleteFn)
}

func (r *baseReconciler) failPublication(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	message string,
) (ctrl.Result, error) {
	plan, err := publicationapp.PlanFailedCatalogStatusMutation(*current, object.GetGeneration(), sourceType, message)
	if err != nil {
		return ctrl.Result{}, err
	}
	return r.applyMutationPlan(ctx, object, current, plan, nil)
}
