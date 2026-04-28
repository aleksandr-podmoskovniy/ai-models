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

package publishobserve

import (
	"fmt"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func ObserveUploadSession(
	request publicationports.Request,
	handle *publicationports.UploadSessionHandle,
	now time.Time,
) (RuntimeObservationDecision, error) {
	if handle == nil {
		return RuntimeObservationDecision{
			Observation: failedObservation("upload session state is missing"),
		}, nil
	}

	input := publicationdomain.UploadSessionObservation{}
	switch {
	case handle.IsComplete():
		rawResult := strings.TrimSpace(handle.TerminationMessage)
		if rawResult == "" {
			input.State = publicationdomain.RuntimeStateFailed
			input.Failure = "upload session completed without a staging result"
			break
		}
		stageHandle, err := decodeUploadStagingHandle(rawResult)
		if err != nil {
			input.State = publicationdomain.RuntimeStateFailed
			input.Failure = err.Error()
			break
		}
		input.State = publicationdomain.RuntimeStateSucceeded
		input.StagedHandle = stageHandle
	case handle.IsFailed():
		input.State = publicationdomain.RuntimeStateFailed
		input.Failure = defaultFailureMessage(handle.TerminationMessage, "upload session failed")
	default:
		input.State = publicationdomain.RuntimeStateRunning
		input.UploadStatus = &handle.UploadStatus
		if handle.UploadStatus.ExpiresAt != nil {
			input.Expired = !handle.UploadStatus.ExpiresAt.Time.After(now.UTC())
		}
	}

	decision, err := publicationdomain.ObserveUploadSession(input)
	if err != nil {
		return RuntimeObservationDecision{}, err
	}

	switch {
	case decision.StagedHandle != nil:
		return RuntimeObservationDecision{
			Observation: publicationdomain.Observation{
				Phase:         publicationdomain.OperationPhaseStaged,
				RuntimeKind:   publicationdomain.RuntimeKindUploadSession,
				CleanupHandle: decision.StagedHandle,
			},
			DeleteRuntime: decision.DeleteSession,
		}, nil
	case decision.FailMessage != "":
		return RuntimeObservationDecision{
			Observation:   failedObservation(decision.FailMessage),
			DeleteRuntime: decision.DeleteSession,
		}, nil
	default:
		return RuntimeObservationDecision{
			Observation: publicationdomain.Observation{
				Phase:       publicationdomain.OperationPhaseRunning,
				RuntimeKind: publicationdomain.RuntimeKindUploadSession,
				Progress:    handle.Progress,
				Upload:      decision.UploadStatus,
			},
		}, nil
	}
}

func decodeUploadStagingHandle(rawResult string) (*cleanuphandle.Handle, error) {
	handle, err := cleanuphandle.Decode(rawResult)
	if err != nil {
		return nil, err
	}
	if handle.Kind != cleanuphandle.KindUploadStaging || handle.UploadStaging == nil {
		return nil, fmt.Errorf("upload session completed without a valid upload staging handle")
	}
	return &handle, nil
}

func failedObservation(message string) publicationdomain.Observation {
	reason := modelsv1alpha1.ModelConditionReasonPublicationFailed
	if storagecapacity.IsInsufficientStorageMessage(message) {
		reason = modelsv1alpha1.ModelConditionReasonInsufficientStorage
	}
	return publicationdomain.Observation{
		Phase:           publicationdomain.OperationPhaseFailed,
		ConditionReason: reason,
		Message:         message,
	}
}

func defaultFailureMessage(message string, fallback string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return fallback
	}
	return message
}
