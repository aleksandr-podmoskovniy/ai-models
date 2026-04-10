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
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	deletionapp "github.com/deckhouse/ai-models/controller/internal/application/deletion"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

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
	if result, handled, err := r.maybeCreateCleanupJob(ctx, object, handle, decision.CreateJob); handled || err != nil {
		return result, err
	}
	if result, handled, err := r.maybeEnsureGarbageCollectionRequest(ctx, object, decision.EnsureGarbageCollectionRequest); handled || err != nil {
		return result, err
	}
	if err := r.maybeUpdateDeleteStatus(ctx, object, decision); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.maybeDeleteGarbageCollectionRequest(ctx, object.GetUID(), decision.DeleteGarbageCollectionRequest); err != nil {
		return ctrl.Result{}, err
	}
	if result, handled, err := r.maybeRemoveDeleteFinalizer(ctx, object, decision.RemoveFinalizer); handled || err != nil {
		return result, err
	}
	return requeueResult(decision.Requeue, r.options.RequeueAfter), nil
}

func (r *baseReconciler) maybeCreateCleanupJob(
	ctx context.Context,
	object client.Object,
	handle cleanuphandle.Handle,
	enabled bool,
) (ctrl.Result, bool, error) {
	if !enabled {
		return ctrl.Result{}, false, nil
	}

	owner, err := cleanupOwnerFor(object)
	if err != nil {
		return ctrl.Result{}, false, err
	}
	options := r.options.CleanupJob
	if handle.Kind == cleanuphandle.KindBackendArtifact {
		projection, err := ociregistry.EnsureProjectedAccess(
			ctx,
			r.client,
			r.scheme,
			object,
			r.options.CleanupJob.Namespace,
			object.GetUID(),
			r.options.CleanupJob.OCIRegistrySecretName,
			r.options.CleanupJob.OCIRegistryCASecretName,
		)
		if err != nil {
			return r.failDeleteStatus(ctx, object, err)
		}
		options.OCIRegistrySecretName = projection.AuthSecretName
		options.OCIRegistryCASecretName = projection.CASecretName
	}
	job, err := buildCleanupJob(owner, handle, options)
	if err != nil {
		return r.failDeleteStatus(ctx, object, err)
	}
	if err := r.client.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, false, err
	}
	return ctrl.Result{}, false, nil
}

func (r *baseReconciler) maybeEnsureGarbageCollectionRequest(
	ctx context.Context,
	object client.Object,
	enabled bool,
) (ctrl.Result, bool, error) {
	if !enabled {
		return ctrl.Result{}, false, nil
	}

	owner, err := cleanupOwnerFor(object)
	if err != nil {
		return ctrl.Result{}, false, err
	}
	if err := r.ensureGarbageCollectionRequest(ctx, owner); err != nil {
		return r.failDeleteStatus(ctx, object, err)
	}
	return ctrl.Result{}, false, nil
}

func (r *baseReconciler) maybeUpdateDeleteStatus(
	ctx context.Context,
	object client.Object,
	decision deletionapp.FinalizeDeleteDecision,
) error {
	if !decision.UpdateStatus {
		return nil
	}
	return r.updateDeleteStatus(ctx, object, decision.StatusReason, decision.StatusMessage)
}

func (r *baseReconciler) maybeDeleteGarbageCollectionRequest(
	ctx context.Context,
	ownerUID types.UID,
	enabled bool,
) error {
	if !enabled {
		return nil
	}
	return r.deleteGarbageCollectionRequest(ctx, ownerUID)
}

func (r *baseReconciler) maybeRemoveDeleteFinalizer(
	ctx context.Context,
	object client.Object,
	enabled bool,
) (ctrl.Result, bool, error) {
	if !enabled {
		return ctrl.Result{}, false, nil
	}
	handle, found, err := cleanuphandle.FromObject(object)
	if err != nil {
		return ctrl.Result{}, true, err
	}
	if found && handle.Kind == cleanuphandle.KindBackendArtifact {
		if err := ociregistry.DeleteProjectedAccess(ctx, r.client, r.options.CleanupJob.Namespace, object.GetUID()); err != nil {
			return ctrl.Result{}, true, err
		}
	}
	controllerutil.RemoveFinalizer(object, Finalizer)
	return ctrl.Result{}, true, r.client.Update(ctx, object)
}

func cleanupOwnerFor(object client.Object) (cleanupJobOwner, error) {
	kind, err := modelobject.KindFor(object)
	if err != nil {
		return cleanupJobOwner{}, err
	}
	return cleanupJobOwner{
		UID:       object.GetUID(),
		Kind:      kind,
		Name:      object.GetName(),
		Namespace: object.GetNamespace(),
	}, nil
}

func requeueResult(enabled bool, after time.Duration) ctrl.Result {
	if !enabled {
		return ctrl.Result{}
	}
	return ctrl.Result{RequeueAfter: after}
}
