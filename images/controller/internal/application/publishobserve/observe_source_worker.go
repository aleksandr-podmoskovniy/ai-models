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
	"errors"
	"strings"

	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
)

type RuntimeObservationDecision struct {
	Observation   publicationdomain.Observation
	DeleteRuntime bool
}

func ObserveSourceWorker(
	request publicationports.Request,
	handle *publicationports.SourceWorkerHandle,
) (RuntimeObservationDecision, error) {
	if handle == nil {
		return RuntimeObservationDecision{
			Observation: failedObservation("source worker pod is missing"),
		}, nil
	}

	input := publicationdomain.SourceWorkerObservation{}
	switch {
	case handle.IsComplete():
		rawResult := strings.TrimSpace(handle.TerminationMessage)
		if rawResult == "" {
			input.State = publicationdomain.RuntimeStateFailed
			input.Failure = "source worker pod completed without a result"
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
		input.Failure = defaultFailureMessage(handle.TerminationMessage, "source worker pod failed")
	default:
		input.State = publicationdomain.RuntimeStateRunning
	}

	decision, err := publicationdomain.ObserveSourceWorker(input)
	if err != nil {
		return RuntimeObservationDecision{}, err
	}
	return mapSourceWorkerDecision(decision)
}

func mapSourceWorkerDecision(
	decision publicationdomain.SourceWorkerDecision,
) (RuntimeObservationDecision, error) {
	switch {
	case decision.Success != nil:
		handle := decision.Success.CleanupHandle
		snapshot := decision.Success.Snapshot
		return RuntimeObservationDecision{
			Observation: publicationdomain.Observation{
				Phase:         publicationdomain.OperationPhaseSucceeded,
				RuntimeKind:   publicationdomain.RuntimeKindSourceWorker,
				Snapshot:      &snapshot,
				CleanupHandle: &handle,
			},
			DeleteRuntime: decision.DeleteWorker,
		}, nil
	case decision.FailMessage != "":
		return RuntimeObservationDecision{
			Observation:   failedObservation(decision.FailMessage),
			DeleteRuntime: decision.DeleteWorker,
		}, nil
	case decision.PersistRunning || decision.RequeueAfter > 0:
		return RuntimeObservationDecision{
			Observation: publicationdomain.Observation{
				Phase:       publicationdomain.OperationPhaseRunning,
				RuntimeKind: publicationdomain.RuntimeKindSourceWorker,
			},
		}, nil
	default:
		return RuntimeObservationDecision{}, errors.New("source worker decision produced no observable state")
	}
}
