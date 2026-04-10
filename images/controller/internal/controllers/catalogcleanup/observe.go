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
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

	if handle.Kind != cleanuphandle.KindBackendArtifact && handle.Kind != cleanuphandle.KindUploadStaging {
		return handle, observation, nil
	}

	jobState, err := r.observeCleanupJobState(ctx, object.GetUID())
	if err != nil {
		return cleanuphandle.Handle{}, deletionapp.FinalizeDeleteInput{}, err
	}
	observation.JobState = jobState
	if jobState == deletionapp.CleanupJobStateComplete {
		gcState, err := r.observeGarbageCollectionState(ctx, object.GetUID())
		if err != nil {
			return cleanuphandle.Handle{}, deletionapp.FinalizeDeleteInput{}, err
		}
		observation.GarbageCollectionState = gcState
	}
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

func (r *baseReconciler) observeGarbageCollectionState(
	ctx context.Context,
	ownerUID types.UID,
) (deletionapp.GarbageCollectionState, error) {
	var secret corev1.Secret
	key := client.ObjectKey{
		Namespace: r.options.CleanupJob.Namespace,
		Name:      dmcrGCRequestSecretName(ownerUID),
	}
	switch err := r.client.Get(ctx, key, &secret); {
	case apierrors.IsNotFound(err):
		return deletionapp.GarbageCollectionStateMissing, nil
	case err != nil:
		return "", err
	default:
		return observeDMCRGCRequestState(&secret), nil
	}
}
