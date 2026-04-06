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
	"github.com/deckhouse/ai-models/controller/internal/controllers/publishrunner"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) ensureOperation(ctx context.Context, request publicationports.Request) (*corev1.ConfigMap, bool, error) {
	name, err := resourcenames.PublicationOperationConfigMapName(request.Owner.UID)
	if err != nil {
		return nil, false, err
	}

	key := client.ObjectKey{Name: name, Namespace: r.options.OperationNamespace}
	var operation corev1.ConfigMap
	switch err := r.client.Get(ctx, key, &operation); {
	case apierrors.IsNotFound(err):
		operation, err := publishrunner.NewConfigMap(r.options.OperationNamespace, request)
		if err != nil {
			return nil, false, err
		}
		if err := r.client.Create(ctx, operation); err != nil {
			return nil, false, err
		}
		return operation, true, nil
	case err != nil:
		return nil, false, err
	default:
		return &operation, false, nil
	}
}

func cleanupHandlePresent(object client.Object) (bool, error) {
	_, found, err := cleanuphandle.FromObject(object)
	return found, err
}

func observationFromConfigMap(operation *corev1.ConfigMap) (publicationdomain.Observation, error) {
	status := publishrunner.StatusFromConfigMap(operation)
	observation := publicationdomain.Observation{
		Phase:   publicationdomain.OperationPhase(status.Phase),
		Message: status.Message,
	}

	switch status.Phase {
	case publicationports.PhasePending, publicationports.PhaseRunning:
		upload, err := publishrunner.UploadStatusFromConfigMap(operation)
		if err != nil {
			return publicationdomain.Observation{}, err
		}
		observation.Upload = upload
	case publicationports.PhaseSucceeded:
		result, err := publishrunner.ResultFromConfigMap(operation)
		if err != nil {
			return publicationdomain.Observation{}, err
		}
		observation.Snapshot = &result.Snapshot
		handle := result.CleanupHandle
		observation.CleanupHandle = &handle
	}

	return observation, nil
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
