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
	auditapp "github.com/deckhouse/ai-models/controller/internal/application/publishaudit"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishobserve"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) cleanupHandlePresent(ctx context.Context, object client.Object) (bool, error) {
	return r.cleanupState.Exists(ctx, object)
}

func (r *baseReconciler) ensureCleanupHandle(ctx context.Context, object client.Object, handle cleanuphandle.Handle) (bool, error) {
	existing, found, err := r.cleanupState.Get(ctx, object)
	if err != nil {
		return false, err
	}
	if found && apiequality.Semantic.DeepEqual(existing, handle) {
		return false, nil
	}

	return r.cleanupState.Ensure(ctx, object, handle)
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

	key := client.ObjectKeyFromObject(object)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := object.DeepCopyObject().(client.Object)
		if err := r.client.Get(ctx, key, latest); err != nil {
			return err
		}

		latestStatus, err := modelobject.GetStatus(latest)
		if err != nil {
			return err
		}
		if apiequality.Semantic.DeepEqual(latestStatus, desired) {
			if err := modelobject.SetStatus(object, desired); err != nil {
				return err
			}
			object.SetResourceVersion(latest.GetResourceVersion())
			*current = desired
			return nil
		}
		if err := modelobject.SetStatus(latest, desired); err != nil {
			return err
		}
		if err := r.client.Status().Update(ctx, latest); err != nil {
			return err
		}

		if err := modelobject.SetStatus(object, desired); err != nil {
			return err
		}
		object.SetResourceVersion(latest.GetResourceVersion())
		*current = desired
		return nil
	})
}

func (r *baseReconciler) applyMutationPlan(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	observation publicationdomain.Observation,
	plan publicationapp.CatalogStatusMutationPlan,
	deleteFn func(context.Context) error,
	uploadSync uploadSessionPhaseSync,
) (ctrl.Result, error) {
	previousStatus := *current
	deleteFn, err := prepareDeleteFn(ctx, plan, deleteFn)
	if err != nil {
		return ctrl.Result{}, err
	}

	requeue, deleteFn, err := r.applyCleanupHandleMutation(ctx, object, plan, deleteFn)
	if err != nil || requeue {
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := runUploadSessionPhaseSync(ctx, uploadSync, true); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.applyStatusMutation(ctx, object, current, previousStatus, sourceType, observation, plan.Status); err != nil {
		return ctrl.Result{}, err
	}
	if err := runUploadSessionPhaseSync(ctx, uploadSync, false); err != nil {
		return ctrl.Result{}, err
	}
	if err := runDeleteFn(ctx, deleteFn); err != nil {
		return ctrl.Result{}, err
	}
	return mutationResult(plan.Requeue), nil
}

func prepareDeleteFn(
	ctx context.Context,
	plan publicationapp.CatalogStatusMutationPlan,
	deleteFn func(context.Context) error,
) (func(context.Context) error, error) {
	if !plan.DeleteRuntime {
		return nil, nil
	}
	if !plan.DeleteRuntimeBeforePersist {
		return deleteFn, nil
	}
	if err := runDeleteFn(ctx, deleteFn); err != nil {
		return nil, err
	}
	return nil, nil
}

func (r *baseReconciler) applyCleanupHandleMutation(
	ctx context.Context,
	object client.Object,
	plan publicationapp.CatalogStatusMutationPlan,
	deleteFn func(context.Context) error,
) (bool, func(context.Context) error, error) {
	if plan.CleanupHandle == nil {
		return false, deleteFn, nil
	}

	updated, err := r.ensureCleanupHandle(ctx, object, *plan.CleanupHandle)
	if err != nil || !updated {
		return false, deleteFn, err
	}

	r.recordAudit(object, auditapp.PlanPreStatusRecords(true, plan.CleanupHandle))
	return true, deleteFn, nil
}

func (r *baseReconciler) applyStatusMutation(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	previous modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	observation publicationdomain.Observation,
	desired modelsv1alpha1.ModelStatus,
) error {
	statusChanged := !apiequality.Semantic.DeepEqual(previous, desired)
	if err := r.updateStatus(ctx, object, current, desired); err != nil {
		return err
	}
	if statusChanged {
		r.recordAudit(object, auditapp.PlanPostStatusRecords(previous, desired, sourceType, observation))
	}
	return nil
}

func runDeleteFn(ctx context.Context, deleteFn func(context.Context) error) error {
	if deleteFn == nil {
		return nil
	}
	return deleteFn(ctx)
}

func mutationResult(requeue bool) ctrl.Result {
	if requeue {
		return ctrl.Result{RequeueAfter: statusPollInterval}
	}
	return ctrl.Result{}
}

func (r *baseReconciler) applyRuntimeObservation(
	ctx context.Context,
	object client.Object,
	spec modelsv1alpha1.ModelSpec,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	mode runtimeMode,
	decision publicationapp.RuntimeObservationDecision,
	deleteFn func(context.Context) error,
) (ctrl.Result, error) {
	plan, err := publicationapp.PlanCatalogStatusMutation(publicationapp.CatalogStatusMutationInput{
		Spec:    spec,
		Current: *current,
		Runtime: publicationapp.CatalogStatusRuntimeResult{
			Generation:    object.GetGeneration(),
			SourceType:    sourceType,
			Observation:   decision.Observation,
			DeleteRuntime: decision.DeleteRuntime,
		},
	})
	if err != nil {
		return r.failPublication(ctx, object, current, sourceType, mode, err.Error())
	}
	uploadSync := r.planUploadSessionPhaseSync(mode, object.GetUID(), sourceType, decision.Observation, plan.Status)
	return r.applyMutationPlan(ctx, object, current, sourceType, decision.Observation, plan, deleteFn, uploadSync)
}

func (r *baseReconciler) failPublication(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	mode runtimeMode,
	message string,
) (ctrl.Result, error) {
	plan, err := publicationapp.PlanFailedCatalogStatusMutation(*current, object.GetGeneration(), sourceType, message)
	if err != nil {
		return ctrl.Result{}, err
	}
	uploadSync := planFailedUploadSessionPhaseSync(r, mode, object.GetUID(), sourceType, plan.Status, message)
	return r.applyMutationPlan(ctx, object, current, sourceType, publicationdomain.Observation{
		Phase:   publicationdomain.OperationPhaseFailed,
		Message: message,
	}, plan, nil, uploadSync)
}

func (r *baseReconciler) failUnsupportedSource(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	message string,
) (ctrl.Result, error) {
	plan, err := publicationapp.PlanUnsupportedSourceCatalogStatusMutation(*current, object.GetGeneration(), message)
	if err != nil {
		return ctrl.Result{}, err
	}
	return r.applyMutationPlan(ctx, object, current, "", publicationdomain.Observation{
		Phase:           publicationdomain.OperationPhaseFailed,
		ConditionReason: modelsv1alpha1.ModelConditionReasonUnsupportedSource,
		Message:         message,
	}, plan, nil, uploadSessionPhaseSync{})
}
