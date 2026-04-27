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

package sourceworker

import (
	"context"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/directuploadstate"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) prepareRequestState(
	ctx context.Context,
	owner client.Object,
	request publicationports.Request,
	plan SourceWorkerPlan,
) (*corev1.Secret, modelpackports.DirectUploadState, error) {
	if err := s.preflight(ctx, request, plan); err != nil {
		return nil, modelpackports.DirectUploadState{}, err
	}
	directUploadStateSecret, err := s.ensureDirectUploadStateSecret(ctx, owner, request.Owner)
	if err != nil {
		return nil, modelpackports.DirectUploadState{}, err
	}
	directUploadState, err := directuploadstate.StateFromSecret(directUploadStateSecret)
	if err != nil {
		return nil, modelpackports.DirectUploadState{}, err
	}
	return directUploadStateSecret, directUploadState, nil
}

func (s *Service) existingOrQueuedHandle(
	ctx context.Context,
	ownerUID types.UID,
	ownerGeneration int64,
	directUploadState modelpackports.DirectUploadState,
) (*publicationports.SourceWorkerHandle, bool, error) {
	existingPod, found, err := s.lookupPod(ctx, ownerUID)
	if err != nil {
		return nil, false, err
	}
	if found {
		if shouldRecreateStalePod(existingPod, ownerGeneration) || shouldRecreateFailedPod(existingPod, directUploadState) {
			if err := ownedresource.DeleteAll(ctx, s.client, existingPod); err != nil {
				return nil, false, err
			}
		} else {
			return s.handleFromPod(existingPod, directUploadState), true, nil
		}
	}

	blocked, err := s.publishConcurrencyBlocked(ctx)
	if err != nil {
		return nil, false, err
	}
	if !blocked {
		return nil, false, nil
	}
	handle, _, err := queuedHandle(ownerUID)
	return handle, true, err
}

func (s *Service) prepareProjectedDependencies(
	ctx context.Context,
	owner client.Object,
	requestOwner publicationports.Owner,
	plan SourceWorkerPlan,
) (Options, string, error) {
	projectedAuthSecretName, err := s.ensureProjectedAuthSecret(ctx, owner, requestOwner, plan)
	if err != nil {
		return Options{}, "", err
	}
	projection, err := ociregistry.EnsureProjectedAccess(
		ctx,
		s.client,
		s.scheme,
		owner,
		s.options.Namespace,
		requestOwner.UID,
		s.options.OCIRegistrySecretName,
		s.options.OCIRegistryCASecretName,
	)
	if err != nil {
		return Options{}, "", err
	}

	options := s.options
	options.OCIRegistrySecretName = projection.AuthSecretName
	options.OCIRegistryCASecretName = projection.CASecretName
	return options, projectedAuthSecretName, nil
}

func isActiveWorkerPhase(phase corev1.PodPhase) bool {
	switch phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return false
	default:
		return true
	}
}

func shouldRecreateFailedPod(
	pod *corev1.Pod,
	directUploadState modelpackports.DirectUploadState,
) bool {
	if pod == nil || pod.Status.Phase != corev1.PodFailed {
		return false
	}
	return directUploadState.Phase == modelpackports.DirectUploadStatePhaseRunning ||
		workerInterruptedByRuntimeLoss(pod)
}

func workerInterruptedByRuntimeLoss(pod *corev1.Pod) bool {
	message := strings.ToLower(terminationMessage(pod, "publish"))
	return strings.Contains(message, "context canceled") ||
		strings.Contains(message, "signal: terminated")
}

func shouldRecreateStalePod(pod *corev1.Pod, ownerGeneration int64) bool {
	if pod == nil {
		return false
	}
	recordedGeneration, err := sourceWorkerOwnerGeneration(pod)
	currentGeneration := normalizedOwnerGeneration(ownerGeneration)
	if err != nil {
		return pod.Status.Phase == corev1.PodSucceeded || currentGeneration > 1
	}
	return recordedGeneration != currentGeneration
}
