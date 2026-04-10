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

package deletion

import (
	"fmt"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type CleanupJobState string

const (
	CleanupJobStateMissing  CleanupJobState = "Missing"
	CleanupJobStateRunning  CleanupJobState = "Running"
	CleanupJobStateComplete CleanupJobState = "Complete"
	CleanupJobStateFailed   CleanupJobState = "Failed"
)

type GarbageCollectionState string

const (
	GarbageCollectionStateMissing   GarbageCollectionState = "Missing"
	GarbageCollectionStateRequested GarbageCollectionState = "Requested"
	GarbageCollectionStateComplete  GarbageCollectionState = "Complete"
)

type FinalizeDeleteInput struct {
	HasFinalizer           bool
	HandleFound            bool
	HandleErr              error
	HandleKind             cleanuphandle.Kind
	JobState               CleanupJobState
	GarbageCollectionState GarbageCollectionState
}

type FinalizeDeleteDecision struct {
	RemoveFinalizer                bool
	CreateJob                      bool
	EnsureGarbageCollectionRequest bool
	DeleteGarbageCollectionRequest bool
	UpdateStatus                   bool
	StatusReason                   modelsv1alpha1.ModelConditionReason
	StatusMessage                  string
	Requeue                        bool
}

func FinalizeDelete(input FinalizeDeleteInput) FinalizeDeleteDecision {
	if !input.HasFinalizer {
		return FinalizeDeleteDecision{}
	}
	if input.HandleErr != nil {
		return failureDecision(
			modelsv1alpha1.ModelConditionReasonCleanupFailed,
			fmt.Sprintf("invalid cleanup handle: %v", input.HandleErr),
		)
	}
	if !input.HandleFound {
		return FinalizeDeleteDecision{RemoveFinalizer: true}
	}

	switch input.HandleKind {
	case cleanuphandle.KindBackendArtifact:
		return finalizeBackendArtifactDelete(input.JobState, input.GarbageCollectionState)
	case cleanuphandle.KindUploadStaging:
		return finalizeUploadStagingDelete(input.JobState)
	default:
		return failureDecision(
			modelsv1alpha1.ModelConditionReasonCleanupBlocked,
			fmt.Sprintf("cleanup handle kind %q is not supported", input.HandleKind),
		)
	}
}

func finalizeUploadStagingDelete(jobState CleanupJobState) FinalizeDeleteDecision {
	switch jobState {
	case CleanupJobStateMissing:
		return FinalizeDeleteDecision{
			CreateJob:     true,
			UpdateStatus:  true,
			StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
			StatusMessage: "upload staging cleanup job created and waiting for completion",
			Requeue:       true,
		}
	case CleanupJobStateRunning:
		return FinalizeDeleteDecision{
			UpdateStatus:  true,
			StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
			StatusMessage: "upload staging cleanup job is still running",
			Requeue:       true,
		}
	case CleanupJobStateFailed:
		return failureDecision(modelsv1alpha1.ModelConditionReasonCleanupFailed, "upload staging cleanup job failed")
	case CleanupJobStateComplete:
		return FinalizeDeleteDecision{RemoveFinalizer: true}
	default:
		return failureDecision(modelsv1alpha1.ModelConditionReasonCleanupFailed, "upload staging cleanup job entered an unsupported state")
	}
}

func finalizeBackendArtifactDelete(
	jobState CleanupJobState,
	garbageCollectionState GarbageCollectionState,
) FinalizeDeleteDecision {
	switch jobState {
	case CleanupJobStateMissing:
		return FinalizeDeleteDecision{
			CreateJob:     true,
			UpdateStatus:  true,
			StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
			StatusMessage: "cleanup job created and waiting for completion",
			Requeue:       true,
		}
	case CleanupJobStateRunning:
		return FinalizeDeleteDecision{
			UpdateStatus:  true,
			StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
			StatusMessage: "cleanup job is still running",
			Requeue:       true,
		}
	case CleanupJobStateFailed:
		return failureDecision(modelsv1alpha1.ModelConditionReasonCleanupFailed, "cleanup job failed")
	case CleanupJobStateComplete:
		switch garbageCollectionState {
		case GarbageCollectionStateMissing:
			return FinalizeDeleteDecision{
				EnsureGarbageCollectionRequest: true,
				UpdateStatus:                   true,
				StatusReason:                   modelsv1alpha1.ModelConditionReasonCleanupPending,
				StatusMessage:                  "registry garbage collection requested",
				Requeue:                        true,
			}
		case GarbageCollectionStateRequested:
			return FinalizeDeleteDecision{
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
				StatusMessage: "registry garbage collection is still running",
				Requeue:       true,
			}
		case GarbageCollectionStateComplete:
			return FinalizeDeleteDecision{
				DeleteGarbageCollectionRequest: true,
				RemoveFinalizer:                true,
			}
		default:
			return failureDecision(modelsv1alpha1.ModelConditionReasonCleanupFailed, "registry garbage collection entered an unsupported state")
		}
	default:
		return failureDecision(modelsv1alpha1.ModelConditionReasonCleanupFailed, "cleanup job entered an unsupported state")
	}
}

func failureDecision(reason modelsv1alpha1.ModelConditionReason, message string) FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		UpdateStatus:  true,
		StatusReason:  reason,
		StatusMessage: message,
		Requeue:       true,
	}
}
