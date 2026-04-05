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
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publication"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	operationRequest publicationports.Request,
) (ctrl.Result, error) {
	if shouldIgnoreObject(object, spec.Source.Type) {
		return ctrl.Result{}, nil
	}

	hasHandle, err := cleanupHandlePresent(object)
	if err != nil {
		return ctrl.Result{}, err
	}
	if shouldSkipReconcile(*status, object.GetGeneration(), hasHandle) {
		return ctrl.Result{}, nil
	}

	operation, created, err := r.ensureOperation(ctx, operationRequest)
	if err != nil {
		return ctrl.Result{}, err
	}
	if created {
		return r.acceptSource(ctx, object, status, spec.Source.Type)
	}

	return r.projectOperationStatus(ctx, object, status, spec.Source.Type, operation)
}

func shouldIgnoreObject(object client.Object, sourceType modelsv1alpha1.ModelSourceType) bool {
	if !object.GetDeletionTimestamp().IsZero() {
		return true
	}
	return !supportsSourceType(sourceType)
}

func (r *baseReconciler) acceptSource(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
) (ctrl.Result, error) {
	desired := publicationdomain.AcceptedStatus(*current, object.GetGeneration(), sourceType)
	if err := r.updateStatus(ctx, object, current, desired); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.options.RequeueAfter}, nil
}

func (r *baseReconciler) projectOperationStatus(
	ctx context.Context,
	object client.Object,
	current *modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	operation *corev1.ConfigMap,
) (ctrl.Result, error) {
	observation, err := observationFromConfigMap(operation)
	if err != nil {
		return r.failPublication(ctx, object, current, sourceType, err.Error())
	}

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
	if projection.Requeue {
		return ctrl.Result{RequeueAfter: r.options.RequeueAfter}, nil
	}
	return ctrl.Result{}, nil
}

func supportsSourceType(sourceType modelsv1alpha1.ModelSourceType) bool {
	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace,
		modelsv1alpha1.ModelSourceTypeUpload,
		modelsv1alpha1.ModelSourceTypeHTTP:
		return true
	default:
		return false
	}
}

func shouldSkipReconcile(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	hasCleanupHandle bool,
) bool {
	if current.ObservedGeneration != generation {
		return false
	}

	switch current.Phase {
	case modelsv1alpha1.ModelPhaseReady:
		return hasCleanupHandle
	case modelsv1alpha1.ModelPhaseFailed:
		return true
	default:
		return false
	}
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
