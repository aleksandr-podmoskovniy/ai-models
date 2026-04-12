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

package publishstate

import (
	"errors"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type Observation struct {
	Phase           OperationPhase
	RuntimeKind     RuntimeKind
	ConditionReason modelsv1alpha1.ModelConditionReason
	Message         string
	Upload          *modelsv1alpha1.ModelUploadStatus
	Snapshot        *publicationdata.Snapshot
	CleanupHandle   *cleanuphandle.Handle
}

type Projection struct {
	Status        modelsv1alpha1.ModelStatus
	Requeue       bool
	CleanupHandle *cleanuphandle.Handle
}

func AcceptedStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
) modelsv1alpha1.ModelStatus {
	if sourceType == modelsv1alpha1.ModelSourceTypeUpload {
		return pendingUploadStatus(current, generation, sourceType)
	}
	return publishingStatus(current, generation, sourceType)
}

func ProjectStatus(
	current modelsv1alpha1.ModelStatus,
	spec modelsv1alpha1.ModelSpec,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	observation Observation,
) (Projection, error) {
	switch observation.Phase {
	case OperationPhasePending, OperationPhaseRunning:
		return Projection{
			Status:  runningStatus(current, generation, sourceType, observation.RuntimeKind, observation.Upload),
			Requeue: true,
		}, nil
	case OperationPhaseStaged:
		if observation.CleanupHandle == nil {
			return Projection{}, errors.New("upload staging cleanup handle must not be empty")
		}
		return Projection{
			Status:        publishingStatus(current, generation, sourceType),
			CleanupHandle: observation.CleanupHandle,
			Requeue:       true,
		}, nil
	case OperationPhaseFailed:
		return Projection{
			Status: failedStatus(current, generation, sourceType, observation.ConditionReason, observation.Message),
		}, nil
	case OperationPhaseSucceeded:
		if observation.Snapshot == nil {
			return Projection{}, errors.New("publication operation result snapshot must not be empty")
		}
		if observation.CleanupHandle == nil {
			return Projection{}, errors.New("publication operation cleanup handle must not be empty")
		}
		return Projection{
			Status:        readyStatus(current, spec, generation, sourceType, *observation.Snapshot),
			CleanupHandle: observation.CleanupHandle,
		}, nil
	default:
		return Projection{}, errors.New("publication operation entered an unsupported phase")
	}
}

func runningStatus(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	runtimeKind RuntimeKind,
	upload *modelsv1alpha1.ModelUploadStatus,
) modelsv1alpha1.ModelStatus {
	if sourceType != modelsv1alpha1.ModelSourceTypeUpload {
		return publishingStatus(current, generation, sourceType)
	}
	if upload != nil {
		return waitForUploadStatus(current, generation, sourceType, upload)
	}
	if runtimeKind == RuntimeKindUploadSession {
		return pendingUploadStatus(current, generation, sourceType)
	}
	return publishingStatus(current, generation, sourceType)
}
