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
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func observeFinalizerGuard(object client.Object) deletionapp.EnsureCleanupFinalizerInput {
	handle, found, err := cleanuphandle.FromObject(object)
	_ = handle
	return deletionapp.EnsureCleanupFinalizerInput{
		HasFinalizer: controllerutil.ContainsFinalizer(object, Finalizer),
		HandleFound:  found,
		HandleErr:    err,
	}
}

func (r *baseReconciler) observeDelete(
	ctx context.Context,
	object client.Object,
) (cleanuphandle.Handle, deletionapp.FinalizeDeleteInput, error) {
	observation := deletionapp.FinalizeDeleteInput{
		HasFinalizer: controllerutil.ContainsFinalizer(object, Finalizer),
	}
	if !observation.HasFinalizer {
		return cleanuphandle.Handle{}, observation, nil
	}

	handle, found, err := cleanuphandle.FromObject(object)
	if err != nil {
		observation.HandleErr = err
		return cleanuphandle.Handle{}, observation, nil
	}
	if !found {
		return cleanuphandle.Handle{}, observation, nil
	}
	observation.HandleFound = true
	observation.HandleKind = handle.Kind

	if handle.Kind != cleanuphandle.KindBackendArtifact {
		return handle, observation, nil
	}

	jobState, err := r.observeCleanupJobState(ctx, object.GetUID())
	if err != nil {
		return cleanuphandle.Handle{}, deletionapp.FinalizeDeleteInput{}, err
	}
	observation.JobState = jobState
	return handle, observation, nil
}

func (r *baseReconciler) observeCleanupJobState(
	ctx context.Context,
	ownerUID types.UID,
) (deletionapp.CleanupJobState, error) {
	jobName, err := resourcenames.CleanupJobName(ownerUID)
	if err != nil {
		return "", err
	}

	jobKey := client.ObjectKey{Namespace: r.options.CleanupJob.Namespace, Name: jobName}
	var job batchv1.Job
	switch err := r.client.Get(ctx, jobKey, &job); {
	case apierrors.IsNotFound(err):
		return deletionapp.CleanupJobStateMissing, nil
	case err != nil:
		return "", err
	case isCleanupJobComplete(&job):
		return deletionapp.CleanupJobStateComplete, nil
	case isCleanupJobFailed(&job):
		return deletionapp.CleanupJobStateFailed, nil
	default:
		return deletionapp.CleanupJobStateRunning, nil
	}
}

func (r *baseReconciler) applyEnsureFinalizerDecision(
	ctx context.Context,
	object client.Object,
	decision deletionapp.EnsureCleanupFinalizerDecision,
) (ctrl.Result, error) {
	switch {
	case decision.RemoveFinalizer:
		controllerutil.RemoveFinalizer(object, Finalizer)
		return ctrl.Result{}, r.client.Update(ctx, object)
	case decision.AddFinalizer:
		controllerutil.AddFinalizer(object, Finalizer)
		return ctrl.Result{}, r.client.Update(ctx, object)
	default:
		return ctrl.Result{}, nil
	}
}

func (r *baseReconciler) applyFinalizeDeleteDecision(
	ctx context.Context,
	object client.Object,
	handle cleanuphandle.Handle,
	decision deletionapp.FinalizeDeleteDecision,
) (ctrl.Result, error) {
	if decision.CreateJob {
		kind, err := modelobject.KindFor(object)
		if err != nil {
			return ctrl.Result{}, err
		}
		job, err := buildCleanupJob(cleanupJobOwner{
			UID:       object.GetUID(),
			Kind:      kind,
			Name:      object.GetName(),
			Namespace: object.GetNamespace(),
		}, handle, r.options.CleanupJob)
		if err != nil {
			if err := r.updateDeleteStatus(ctx, object, modelsv1alpha1.ModelConditionReasonCleanupFailed, err.Error()); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: r.options.RequeueAfter}, nil
		}
		if err := r.client.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
	}
	if decision.UpdateStatus {
		if err := r.updateDeleteStatus(ctx, object, decision.StatusReason, decision.StatusMessage); err != nil {
			return ctrl.Result{}, err
		}
	}
	if decision.RemoveFinalizer {
		controllerutil.RemoveFinalizer(object, Finalizer)
		return ctrl.Result{}, r.client.Update(ctx, object)
	}
	if decision.Requeue {
		return ctrl.Result{RequeueAfter: r.options.RequeueAfter}, nil
	}
	return ctrl.Result{}, nil
}

func (r *baseReconciler) updateDeleteStatus(
	ctx context.Context,
	object client.Object,
	reason modelsv1alpha1.ModelConditionReason,
	message string,
) error {
	status, err := modelobject.GetStatus(object)
	if err != nil {
		return err
	}
	status.Phase = modelsv1alpha1.ModelPhaseDeleting
	status.ObservedGeneration = object.GetGeneration()
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionCleanupCompleted),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: object.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	})
	if err := modelobject.SetStatus(object, status); err != nil {
		return err
	}
	return r.client.Status().Update(ctx, object)
}
