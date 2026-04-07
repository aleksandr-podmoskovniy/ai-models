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
	"strings"
	"time"

	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
)

func ObserveUploadSession(
	request publicationports.Request,
	handle *publicationports.UploadSessionHandle,
	now time.Time,
) (RuntimeObservationDecision, error) {
	if handle == nil {
		return RuntimeObservationDecision{
			Observation: failedObservation("upload session worker pod is missing"),
		}, nil
	}

	input := publicationdomain.UploadSessionObservation{}
	switch {
	case handle.IsComplete():
		rawResult := strings.TrimSpace(handle.TerminationMessage)
		if rawResult == "" {
			input.State = publicationdomain.RuntimeStateFailed
			input.Failure = "upload session completed without a publication result"
			break
		}
		success, err := decodeRuntimeResult(request, rawResult)
		if err != nil {
			input.State = publicationdomain.RuntimeStateFailed
			input.Failure = err.Error()
			break
		}
		input.State = publicationdomain.RuntimeStateSucceeded
		input.Success = success
	case handle.IsFailed():
		input.State = publicationdomain.RuntimeStateFailed
		input.Failure = defaultFailureMessage(handle.TerminationMessage, "upload session worker pod failed")
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
	case decision.Success != nil:
		handle := decision.Success.CleanupHandle
		snapshot := decision.Success.Snapshot
		return RuntimeObservationDecision{
			Observation: publicationdomain.Observation{
				Phase:         publicationdomain.OperationPhaseSucceeded,
				Snapshot:      &snapshot,
				CleanupHandle: &handle,
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
				Phase:  publicationdomain.OperationPhaseRunning,
				Upload: decision.UploadStatus,
			},
		}, nil
	}
}

func failedObservation(message string) publicationdomain.Observation {
	return publicationdomain.Observation{
		Phase:   publicationdomain.OperationPhaseFailed,
		Message: message,
	}
}

func defaultFailureMessage(message string, fallback string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return fallback
	}
	return message
}
