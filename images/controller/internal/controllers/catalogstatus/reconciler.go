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
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishobserve"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const statusPollInterval = time.Second

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var object modelsv1alpha1.Model
	if err := r.client.Get(ctx, req.NamespacedName, &object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	operationRequest, err := modelobject.PublicationRequest(&object, object.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.reconcileObject(ctx, &object, object.Spec, &object.Status, operationRequest)
}

func (r *ClusterModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var object modelsv1alpha1.ClusterModel
	if err := r.client.Get(ctx, req.NamespacedName, &object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	operationRequest, err := modelobject.PublicationRequest(&object, object.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.reconcileObject(ctx, &object, object.Spec, &object.Status, operationRequest)
}

func (r *baseReconciler) reconcileObject(
	ctx context.Context,
	object client.Object,
	spec modelsv1alpha1.ModelSpec,
	status *modelsv1alpha1.ModelStatus,
	request publicationports.Request,
) (ctrl.Result, error) {
	hasHandle, err := cleanupHandlePresent(object)
	if err != nil {
		return ctrl.Result{}, err
	}

	decision, err := publicationapp.DecideCatalogStatusReconcile(publicationapp.CatalogStatusReconcileInput{
		Deleting:           !object.GetDeletionTimestamp().IsZero(),
		Source:             spec.Source,
		UploadStagePresent: request.UploadStage != nil,
		Current:            *status,
		Generation:         object.GetGeneration(),
		HasCleanupHandle:   hasHandle,
	})
	if err != nil {
		if modelsv1alpha1.IsUnsupportedRemoteSourceError(err) {
			return r.failUnsupportedSource(ctx, object, status, err.Error())
		}
		return ctrl.Result{}, err
	}
	if decision.Skip {
		return ctrl.Result{}, nil
	}

	result, err := publicationapp.EnsureRuntimeObservation(publicationapp.EnsureRuntimeObservationInput{
		Context:        ctx,
		Owner:          object,
		Request:        request,
		Mode:           decision.Mode,
		SourceWorkers:  r.sourceWorkers,
		UploadSessions: r.uploadSessions,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		return r.failPublication(ctx, object, status, decision.SourceType, decision.Mode, err.Error())
	}
	return r.applyRuntimeObservation(ctx, object, spec, status, decision.SourceType, decision.Mode, result.Decision, result.DeleteFn)
}
