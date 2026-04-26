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

	deletionapp "github.com/deckhouse/ai-models/controller/internal/application/deletion"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *baseReconciler) observeFinalizerGuard(ctx context.Context, object client.Object) deletionapp.EnsureCleanupFinalizerInput {
	handle, found, err := r.cleanupState.Get(ctx, object)
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

	handle, found, err := r.cleanupState.Get(ctx, object)
	if err != nil {
		observation.HandleErr = err
		return cleanuphandle.Handle{}, observation, nil
	}
	if !found {
		runtimeResources, err := r.observePublicationRuntimeResources(ctx, object)
		if err != nil {
			return cleanuphandle.Handle{}, deletionapp.FinalizeDeleteInput{}, err
		}
		observation.RuntimeResourcePresent = runtimeResources.Present()
		return cleanuphandle.Handle{}, observation, nil
	}
	observation.HandleFound = true
	observation.HandleKind = handle.Kind

	if !needsCleanupObservation(handle) {
		return handle, observation, nil
	}

	cleanupState, err := r.observeCleanupState(ctx, object)
	if err != nil {
		return cleanuphandle.Handle{}, deletionapp.FinalizeDeleteInput{}, err
	}
	observation.CleanupState = cleanupState
	if needsGarbageCollectionObservation(handle, cleanupState) {
		gcState, err := r.observeGarbageCollectionState(ctx, object.GetUID())
		if err != nil {
			return cleanuphandle.Handle{}, deletionapp.FinalizeDeleteInput{}, err
		}
		observation.GarbageCollectionState = gcState
	}
	return handle, observation, nil
}

func (r *baseReconciler) observeFinalizeDeleteFlow(
	ctx context.Context,
	object client.Object,
) (finalizeDeleteFlow, error) {
	handle, observation, err := r.observeDelete(ctx, object)
	if err != nil {
		return finalizeDeleteFlow{}, err
	}

	decision := deletionapp.FinalizeDelete(observation)
	runtime, err := buildFinalizeDeleteRuntime(object, handle, decision)
	if err != nil {
		return finalizeDeleteFlow{}, err
	}

	return finalizeDeleteFlow{
		runtime:  runtime,
		decision: decision,
	}, nil
}

func needsCleanupObservation(handle cleanuphandle.Handle) bool {
	return handle.Kind == cleanuphandle.KindBackendArtifact || handle.Kind == cleanuphandle.KindUploadStaging
}

func needsGarbageCollectionObservation(
	handle cleanuphandle.Handle,
	cleanupState deletionapp.CleanupOperationState,
) bool {
	return handle.Kind == cleanuphandle.KindBackendArtifact && cleanupState == deletionapp.CleanupOperationStateComplete
}

func (r *baseReconciler) observeCleanupState(
	ctx context.Context,
	object client.Object,
) (deletionapp.CleanupOperationState, error) {
	completed, err := r.cleanupState.Completed(ctx, object)
	if err != nil {
		return "", err
	}
	if completed {
		return deletionapp.CleanupOperationStateComplete, nil
	}
	return deletionapp.CleanupOperationStateMissing, nil
}

func (r *baseReconciler) observeGarbageCollectionState(
	ctx context.Context,
	ownerUID types.UID,
) (deletionapp.GarbageCollectionState, error) {
	var secret corev1.Secret
	key := garbageCollectionRequestKey(r.options.Cleanup.Namespace, ownerUID)
	switch err := r.client.Get(ctx, key, &secret); {
	case apierrors.IsNotFound(err):
		return deletionapp.GarbageCollectionStateMissing, nil
	case err != nil:
		return "", err
	default:
		return observeDMCRGCRequestState(&secret), nil
	}
}
