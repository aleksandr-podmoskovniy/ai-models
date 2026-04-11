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

type cleanupJobMessages struct {
	created     string
	running     string
	failed      string
	unsupported string
}

func finalizeUploadStagingDelete(jobState CleanupJobState) FinalizeDeleteDecision {
	decision, terminal := cleanupJobProgressDecision(jobState, cleanupJobMessages{
		created:     "upload staging cleanup job created and waiting for completion",
		running:     "upload staging cleanup job is still running",
		failed:      "upload staging cleanup job failed",
		unsupported: "upload staging cleanup job entered an unsupported state",
	})
	if terminal {
		return decision
	}
	return removeFinalizerDecision()
}

func finalizeBackendArtifactDelete(
	jobState CleanupJobState,
	garbageCollectionState GarbageCollectionState,
) FinalizeDeleteDecision {
	decision, terminal := cleanupJobProgressDecision(jobState, cleanupJobMessages{
		created:     "cleanup job created and waiting for completion",
		running:     "cleanup job is still running",
		failed:      "cleanup job failed",
		unsupported: "cleanup job entered an unsupported state",
	})
	if terminal {
		return decision
	}
	return garbageCollectionProgressDecision(garbageCollectionState)
}

func cleanupJobProgressDecision(jobState CleanupJobState, messages cleanupJobMessages) (FinalizeDeleteDecision, bool) {
	switch jobState {
	case CleanupJobStateMissing:
		return createJobDecision(messages.created), true
	case CleanupJobStateRunning:
		return pendingDecision(messages.running), true
	case CleanupJobStateFailed:
		return failureDecision(modelsv1alpha1.ModelConditionReasonCleanupFailed, messages.failed), true
	case CleanupJobStateComplete:
		return FinalizeDeleteDecision{}, false
	default:
		return failureDecision(modelsv1alpha1.ModelConditionReasonCleanupFailed, messages.unsupported), true
	}
}

func garbageCollectionProgressDecision(state GarbageCollectionState) FinalizeDeleteDecision {
	switch state {
	case GarbageCollectionStateMissing:
		return ensureGarbageCollectionDecision("registry garbage collection requested")
	case GarbageCollectionStateRequested:
		return pendingDecision("registry garbage collection is still running")
	case GarbageCollectionStateComplete:
		return deleteGarbageCollectionAndRemoveFinalizerDecision()
	default:
		return failureDecision(modelsv1alpha1.ModelConditionReasonCleanupFailed, "registry garbage collection entered an unsupported state")
	}
}

func createJobDecision(message string) FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		CreateJob:     true,
		UpdateStatus:  true,
		StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
		StatusMessage: message,
		Requeue:       true,
	}
}

func ensureGarbageCollectionDecision(message string) FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		EnsureGarbageCollectionRequest: true,
		UpdateStatus:                   true,
		StatusReason:                   modelsv1alpha1.ModelConditionReasonCleanupPending,
		StatusMessage:                  message,
		Requeue:                        true,
	}
}

func pendingDecision(message string) FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		UpdateStatus:  true,
		StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
		StatusMessage: message,
		Requeue:       true,
	}
}

func removeFinalizerDecision() FinalizeDeleteDecision {
	return FinalizeDeleteDecision{RemoveFinalizer: true}
}

func deleteGarbageCollectionAndRemoveFinalizerDecision() FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		DeleteGarbageCollectionRequest: true,
		RemoveFinalizer:                true,
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
