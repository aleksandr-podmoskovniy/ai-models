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
	"github.com/deckhouse/ai-models/controller/internal/artifactbackend"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
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

func observationFromRuntimeResult(
	request publicationports.Request,
	rawResult string,
) (publicationdomain.Observation, error) {
	backendResult, err := artifactbackend.DecodeResult(rawResult)
	if err != nil {
		return publicationdomain.Observation{}, err
	}

	snapshot := publication.Snapshot{
		Identity: request.Identity,
		Source:   backendResult.Source,
		Artifact: backendResult.Artifact,
		Resolved: backendResult.Resolved,
		Result: publication.Result{
			State: "Published",
			Ready: true,
		},
	}

	handle := backendResult.CleanupHandle
	return publicationdomain.Observation{
		Phase:         publicationdomain.OperationPhaseSucceeded,
		Snapshot:      &snapshot,
		CleanupHandle: &handle,
	}, nil
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

func (r *baseReconciler) projectObservation(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	observation publicationdomain.Observation,
	deleteFn func(context.Context) error,
) (ctrl.Result, error) {
	projection, err := publicationdomain.ProjectStatus(*current, object.GetGeneration(), sourceType, observation)
	if err != nil {
		return r.failPublication(ctx, object, current, sourceType, err.Error())
	}
	if projection.CleanupHandle != nil {
		updated, err := r.ensureCleanupHandle(ctx, object, *projection.CleanupHandle)
		if err != nil {
			return ctrl.Result{}, err
		}
		if updated {
			return ctrl.Result{Requeue: true}, nil
		}
	}
	if err := r.updateStatus(ctx, object, current, projection.Status); err != nil {
		return ctrl.Result{}, err
	}
	if deleteFn != nil {
		if err := deleteFn(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}
	if projection.Requeue {
		return ctrl.Result{RequeueAfter: statusPollInterval}, nil
	}
	return ctrl.Result{}, nil
}

func (r *baseReconciler) failAndDelete(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	message string,
	deleteFn func(context.Context) error,
) (ctrl.Result, error) {
	if deleteFn != nil {
		if err := deleteFn(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}
	return r.failPublication(ctx, object, current, sourceType, message)
}

func (r *baseReconciler) failPublication(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	message string,
) (ctrl.Result, error) {
	projection, err := publicationdomain.ProjectStatus(*current, object.GetGeneration(), sourceType, publicationdomain.Observation{
		Phase:   publicationdomain.OperationPhaseFailed,
		Message: message,
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.updateStatus(ctx, object, current, projection.Status); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
