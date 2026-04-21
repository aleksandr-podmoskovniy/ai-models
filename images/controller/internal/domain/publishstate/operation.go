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

import modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"

type OperationPhase string

const (
	OperationPhasePending   OperationPhase = "Pending"
	OperationPhaseRunning   OperationPhase = "Running"
	OperationPhaseStaged    OperationPhase = "Staged"
	OperationPhaseFailed    OperationPhase = "Failed"
	OperationPhaseSucceeded OperationPhase = "Succeeded"
)

type RuntimeState string

const (
	RuntimeStateRunning        RuntimeState = "Running"
	RuntimeStateAwaitingResult RuntimeState = "AwaitingResult"
	RuntimeStateSucceeded      RuntimeState = "Succeeded"
	RuntimeStateFailed         RuntimeState = "Failed"
)

type OperationStatusView struct {
	Phase      OperationPhase
	WorkerName string
}

type RuntimeKind string

const (
	RuntimeKindSourceWorker  RuntimeKind = "SourceWorker"
	RuntimeKindUploadSession RuntimeKind = "UploadSession"
)

func IsTerminalOperationPhase(phase OperationPhase) bool {
	switch phase {
	case OperationPhaseSucceeded, OperationPhaseFailed:
		return true
	default:
		return false
	}
}

func SameUploadStatus(current, desired *modelsv1alpha1.ModelUploadStatus) bool {
	switch {
	case current == nil && desired == nil:
		return true
	case current == nil || desired == nil:
		return false
	}

	if current.Repository != desired.Repository ||
		current.ExternalURL != desired.ExternalURL ||
		current.InClusterURL != desired.InClusterURL ||
		current.AuthorizationHeaderValue != desired.AuthorizationHeaderValue {
		return false
	}
	switch {
	case current.ExpiresAt == nil && desired.ExpiresAt == nil:
		return true
	case current.ExpiresAt == nil || desired.ExpiresAt == nil:
		return false
	default:
		return current.ExpiresAt.Equal(desired.ExpiresAt)
	}
}
