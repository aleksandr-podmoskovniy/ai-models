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
	GarbageCollectionStateQueued    GarbageCollectionState = "Queued"
	GarbageCollectionStateRequested GarbageCollectionState = "Requested"
	GarbageCollectionStateComplete  GarbageCollectionState = "Complete"
)

type FinalizeDeleteInput struct {
	HasFinalizer           bool
	HandleFound            bool
	HandleErr              error
	HandleKind             cleanuphandle.Kind
	RuntimeResourcePresent bool
	JobState               CleanupJobState
	GarbageCollectionState GarbageCollectionState
}

type FinalizeDeleteDecision struct {
	RemoveFinalizer                bool
	DeleteRuntimeResources         bool
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
			modelsv1alpha1.ModelConditionReasonFailed,
			fmt.Sprintf("invalid cleanup handle: %v", input.HandleErr),
		)
	}
	if !input.HandleFound {
		if input.RuntimeResourcePresent {
			return pendingRuntimeCleanupDecision()
		}
		return FinalizeDeleteDecision{RemoveFinalizer: true}
	}

	switch input.HandleKind {
	case cleanuphandle.KindBackendArtifact:
		return finalizeBackendArtifactDelete(input.JobState, input.GarbageCollectionState)
	case cleanuphandle.KindUploadStaging:
		return finalizeUploadStagingDelete(input.JobState)
	default:
		return failureDecision(
			modelsv1alpha1.ModelConditionReasonFailed,
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
		return failureDecision(modelsv1alpha1.ModelConditionReasonFailed, messages.failed), true
	case CleanupJobStateComplete:
		return FinalizeDeleteDecision{}, false
	default:
		return failureDecision(modelsv1alpha1.ModelConditionReasonFailed, messages.unsupported), true
	}
}

func garbageCollectionProgressDecision(state GarbageCollectionState) FinalizeDeleteDecision {
	switch state {
	case GarbageCollectionStateMissing:
		return enqueueGarbageCollectionAndRemoveFinalizerDecision()
	case GarbageCollectionStateQueued, GarbageCollectionStateRequested, GarbageCollectionStateComplete:
		return removeFinalizerDecision()
	default:
		return failureDecision(modelsv1alpha1.ModelConditionReasonFailed, "registry garbage collection entered an unsupported state")
	}
}

func createJobDecision(message string) FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		CreateJob:     true,
		UpdateStatus:  true,
		StatusReason:  modelsv1alpha1.ModelConditionReasonPending,
		StatusMessage: message,
		Requeue:       true,
	}
}

func pendingRuntimeCleanupDecision() FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		DeleteRuntimeResources: true,
		UpdateStatus:           true,
		StatusReason:           modelsv1alpha1.ModelConditionReasonPending,
		StatusMessage:          "publication runtime resources are still being cleaned up",
		Requeue:                true,
	}
}

func enqueueGarbageCollectionAndRemoveFinalizerDecision() FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		EnsureGarbageCollectionRequest: true,
		RemoveFinalizer:                true,
	}
}

func pendingDecision(message string) FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		UpdateStatus:  true,
		StatusReason:  modelsv1alpha1.ModelConditionReasonPending,
		StatusMessage: message,
		Requeue:       true,
	}
}

func removeFinalizerDecision() FinalizeDeleteDecision {
	return FinalizeDeleteDecision{RemoveFinalizer: true}
}

func failureDecision(reason modelsv1alpha1.ModelConditionReason, message string) FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		UpdateStatus:  true,
		StatusReason:  reason,
		StatusMessage: message,
		Requeue:       true,
	}
}
