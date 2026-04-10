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
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type PublicationSuccess struct {
	Snapshot      publicationdata.Snapshot
	CleanupHandle cleanuphandle.Handle
}

type SourceWorkerObservation struct {
	Current      OperationStatusView
	WorkerName   string
	Created      bool
	State        RuntimeState
	Failure      string
	Success      *PublicationSuccess
	RequeueAfter time.Duration
}

type SourceWorkerDecision struct {
	PersistRunning bool
	RunningWorker  string
	Success        *PublicationSuccess
	FailMessage    string
	DeleteWorker   bool
	RequeueAfter   time.Duration
}

type UploadSessionObservation struct {
	Current       OperationStatusView
	WorkerName    string
	Created       bool
	State         RuntimeState
	Failure       string
	StagedHandle  *cleanuphandle.Handle
	CurrentUpload *modelsv1alpha1.ModelUploadStatus
	UploadStatus  *modelsv1alpha1.ModelUploadStatus
	Expired       bool
	RequeueAfter  time.Duration
}

type UploadSessionDecision struct {
	PersistRunning bool
	RunningWorker  string
	PersistUpload  bool
	UploadStatus   *modelsv1alpha1.ModelUploadStatus
	StagedHandle   *cleanuphandle.Handle
	FailMessage    string
	DeleteSession  bool
	RequeueAfter   time.Duration
}

func ObserveSourceWorker(observation SourceWorkerObservation) (SourceWorkerDecision, error) {
	switch observation.State {
	case RuntimeStateRunning:
		decision := SourceWorkerDecision{}
		if observation.Created ||
			observation.Current.Phase != OperationPhaseRunning ||
			observation.Current.WorkerName != observation.WorkerName {
			decision.PersistRunning = true
			decision.RunningWorker = observation.WorkerName
		}
		return decision, nil
	case RuntimeStateAwaitingResult:
		return SourceWorkerDecision{RequeueAfter: observation.RequeueAfter}, nil
	case RuntimeStateSucceeded:
		if observation.Success == nil {
			return SourceWorkerDecision{}, errors.New("source worker success payload must not be empty")
		}
		return SourceWorkerDecision{
			Success:      observation.Success,
			DeleteWorker: true,
		}, nil
	case RuntimeStateFailed:
		if observation.Failure == "" {
			return SourceWorkerDecision{}, errors.New("source worker failure message must not be empty")
		}
		return SourceWorkerDecision{
			FailMessage:  observation.Failure,
			DeleteWorker: true,
		}, nil
	default:
		return SourceWorkerDecision{}, errors.New("source worker entered an unsupported state")
	}
}

func ObserveUploadSession(observation UploadSessionObservation) (UploadSessionDecision, error) {
	switch observation.State {
	case RuntimeStateRunning:
		if observation.Expired {
			return UploadSessionDecision{
				FailMessage:   "upload session expired before publication completed",
				DeleteSession: true,
			}, nil
		}
		decision := UploadSessionDecision{
			RequeueAfter: observation.RequeueAfter,
		}
		if observation.Created ||
			observation.Current.Phase != OperationPhaseRunning ||
			observation.Current.WorkerName != observation.WorkerName {
			decision.PersistRunning = true
			decision.RunningWorker = observation.WorkerName
		}
		if !SameUploadStatus(observation.CurrentUpload, observation.UploadStatus) {
			decision.PersistUpload = true
			decision.UploadStatus = observation.UploadStatus
		}
		return decision, nil
	case RuntimeStateSucceeded:
		if observation.StagedHandle == nil {
			return UploadSessionDecision{}, errors.New("upload session staging handle must not be empty")
		}
		return UploadSessionDecision{
			StagedHandle:  observation.StagedHandle,
			DeleteSession: true,
		}, nil
	case RuntimeStateFailed:
		if observation.Failure == "" {
			return UploadSessionDecision{}, errors.New("upload session failure message must not be empty")
		}
		return UploadSessionDecision{
			FailMessage:   observation.Failure,
			DeleteSession: true,
		}, nil
	default:
		return UploadSessionDecision{}, errors.New("upload session entered an unsupported state")
	}
}
