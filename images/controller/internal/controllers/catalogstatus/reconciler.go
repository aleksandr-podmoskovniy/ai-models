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
	"fmt"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishobserve"
	"github.com/deckhouse/ai-models/controller/internal/domain/modelsource"
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
	uploadStage, err := r.cleanupState.UploadStage(ctx, &object)
	if err != nil {
		return ctrl.Result{}, err
	}
	operationRequest, err := modelobject.PublicationRequest(&object, object.Spec, uploadStage)
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
	uploadStage, err := r.cleanupState.UploadStage(ctx, &object)
	if err != nil {
		return ctrl.Result{}, err
	}
	spec, err := modelobject.SpecFor(&object)
	if err != nil {
		return ctrl.Result{}, err
	}
	operationRequest, err := modelobject.PublicationRequest(&object, spec, uploadStage)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.reconcileObject(ctx, &object, spec, &object.Status, operationRequest)
}

func (r *baseReconciler) reconcileObject(
	ctx context.Context,
	object client.Object,
	spec modelsv1alpha1.ModelSpec,
	status *modelsv1alpha1.ModelStatus,
	request publicationports.Request,
) (ctrl.Result, error) {
	hasHandle, err := r.cleanupHandlePresent(ctx, object)
	if err != nil {
		return ctrl.Result{}, err
	}

	decision, err := decideCatalogStatusReconcile(
		!object.GetDeletionTimestamp().IsZero(),
		spec.Source,
		request.UploadStage != nil,
		*status,
		object.GetGeneration(),
		hasHandle,
	)
	if err != nil {
		if modelsource.IsUnsupportedRemoteError(err) {
			return r.failUnsupportedSource(ctx, object, status, err.Error())
		}
		return ctrl.Result{}, err
	}
	if decision.Skip {
		return ctrl.Result{}, nil
	}

	result, err := r.ensureRuntimeObservation(ctx, object, request, decision.Mode)
	if err != nil {
		return r.failPublication(ctx, object, status, decision.SourceType, decision.Mode, err.Error())
	}
	return r.applyRuntimeObservation(ctx, object, spec, status, decision.SourceType, decision.Mode, result.Decision, result.DeleteFn)
}

func (r *baseReconciler) ensureRuntimeObservation(
	ctx context.Context,
	object client.Object,
	request publicationports.Request,
	mode runtimeMode,
) (runtimeObservationResult, error) {
	switch mode {
	case runtimeModeSourceWorker:
		return r.ensureSourceWorkerObservation(ctx, object, request)
	case runtimeModeUpload:
		return r.ensureUploadSessionObservation(ctx, object, request)
	default:
		return runtimeObservationResult{}, fmt.Errorf("unsupported publication runtime mode %q", mode)
	}
}

type runtimeObservationResult struct {
	Decision publicationapp.RuntimeObservationDecision
	DeleteFn func(context.Context) error
}

func (r *baseReconciler) ensureSourceWorkerObservation(
	ctx context.Context,
	object client.Object,
	request publicationports.Request,
) (runtimeObservationResult, error) {
	if r.sourceWorkers == nil {
		return runtimeObservationResult{}, fmt.Errorf("source worker runtime must not be nil")
	}

	handle, _, err := r.sourceWorkers.GetOrCreate(ctx, object, request)
	if err != nil {
		return runtimeObservationResult{}, err
	}

	decision, err := publicationapp.ObserveSourceWorker(request, handle)
	if err != nil {
		return runtimeObservationResult{}, err
	}

	var deleteFn func(context.Context) error
	if handle != nil {
		deleteFn = handle.Delete
	}
	return runtimeObservationResult{Decision: decision, DeleteFn: deleteFn}, nil
}

func (r *baseReconciler) ensureUploadSessionObservation(
	ctx context.Context,
	object client.Object,
	request publicationports.Request,
) (runtimeObservationResult, error) {
	if r.uploadSessions == nil {
		return runtimeObservationResult{}, fmt.Errorf("upload session runtime must not be nil")
	}

	handle, _, err := r.uploadSessions.GetOrCreate(ctx, object, request)
	if err != nil {
		return runtimeObservationResult{}, err
	}

	decision, err := publicationapp.ObserveUploadSession(request, handle, time.Now().UTC())
	if err != nil {
		return runtimeObservationResult{}, err
	}

	var deleteFn func(context.Context) error
	if handle != nil {
		deleteFn = handle.Delete
	}
	return runtimeObservationResult{Decision: decision, DeleteFn: deleteFn}, nil
}
