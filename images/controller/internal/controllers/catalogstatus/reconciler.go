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
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
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
	sourceType, err := spec.Source.DetectType()
	if err != nil {
		return ctrl.Result{}, err
	}

	if shouldIgnoreObject(object, sourceType) {
		return ctrl.Result{}, nil
	}

	hasHandle, err := cleanupHandlePresent(object)
	if err != nil {
		return ctrl.Result{}, err
	}
	if shouldSkipReconcile(*status, object.GetGeneration(), hasHandle) {
		return ctrl.Result{}, nil
	}

	if sourceType == modelsv1alpha1.ModelSourceTypeUpload {
		return r.reconcileUploadSession(ctx, object, status, sourceType, request)
	}
	return r.reconcileSourceWorker(ctx, object, status, sourceType, request)
}

func (r *baseReconciler) reconcileSourceWorker(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	request publicationports.Request,
) (ctrl.Result, error) {
	worker, _, err := r.sourceWorkers.GetOrCreate(ctx, object, publicationports.OperationContext{
		Request: request,
	})
	if err != nil {
		return r.failPublication(ctx, object, current, sourceType, err.Error())
	}

	switch {
	case worker.IsComplete():
		rawResult := strings.TrimSpace(worker.TerminationMessage)
		if rawResult == "" {
			return r.failAndDelete(ctx, object, current, sourceType, "publication worker pod completed without a result", worker.Delete)
		}
		observation, err := observationFromRuntimeResult(request, rawResult)
		if err != nil {
			return r.failAndDelete(ctx, object, current, sourceType, err.Error(), worker.Delete)
		}
		return r.projectObservation(ctx, object, current, sourceType, observation, worker.Delete)
	case worker.IsFailed():
		message := strings.TrimSpace(worker.TerminationMessage)
		if message == "" {
			message = "publication worker pod failed"
		}
		return r.failAndDelete(ctx, object, current, sourceType, message, worker.Delete)
	default:
		return r.projectObservation(ctx, object, current, sourceType, publicationdomain.Observation{
			Phase: publicationdomain.OperationPhaseRunning,
		}, nil)
	}
}

func (r *baseReconciler) reconcileUploadSession(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	request publicationports.Request,
) (ctrl.Result, error) {
	session, _, err := r.uploadSessions.GetOrCreate(ctx, object, publicationports.OperationContext{
		Request: request,
	})
	if err != nil {
		return r.failPublication(ctx, object, current, sourceType, err.Error())
	}
	if session == nil {
		return r.failPublication(ctx, object, current, sourceType, "upload session worker pod is missing")
	}

	switch {
	case session.IsComplete():
		rawResult := strings.TrimSpace(session.TerminationMessage)
		if rawResult == "" {
			return r.failAndDelete(ctx, object, current, sourceType, "upload session completed without a publication result", session.Delete)
		}
		observation, err := observationFromRuntimeResult(request, rawResult)
		if err != nil {
			return r.failAndDelete(ctx, object, current, sourceType, err.Error(), session.Delete)
		}
		return r.projectObservation(ctx, object, current, sourceType, observation, session.Delete)
	case session.IsFailed():
		message := strings.TrimSpace(session.TerminationMessage)
		if message == "" {
			message = "upload session worker pod failed"
		}
		return r.failAndDelete(ctx, object, current, sourceType, message, session.Delete)
	default:
		if session.UploadStatus.ExpiresAt != nil {
			now := time.Now().UTC()
			if !session.UploadStatus.ExpiresAt.Time.After(now) {
				return r.failAndDelete(ctx, object, current, sourceType, "upload session expired before publication completed", session.Delete)
			}
		}
		return r.projectObservation(ctx, object, current, sourceType, publicationdomain.Observation{
			Phase:  publicationdomain.OperationPhaseRunning,
			Upload: &session.UploadStatus,
		}, nil)
	}
}
