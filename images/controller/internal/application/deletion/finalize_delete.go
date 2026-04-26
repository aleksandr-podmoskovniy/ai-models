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

type CleanupOperationState string

const (
	CleanupOperationStateMissing  CleanupOperationState = "Missing"
	CleanupOperationStateComplete CleanupOperationState = "Complete"
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
	CleanupState           CleanupOperationState
	GarbageCollectionState GarbageCollectionState
}

type FinalizeDeleteDecision struct {
	RemoveFinalizer                bool
	DeleteRuntimeResources         bool
	RunCleanup                     bool
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
		return finalizeBackendArtifactDelete(input.CleanupState, input.GarbageCollectionState)
	case cleanuphandle.KindUploadStaging:
		return finalizeUploadStagingDelete(input.CleanupState)
	default:
		return failureDecision(
			modelsv1alpha1.ModelConditionReasonFailed,
			fmt.Sprintf("cleanup handle kind %q is not supported", input.HandleKind),
		)
	}
}

type cleanupOperationMessages struct {
	started     string
	unsupported string
}

func finalizeUploadStagingDelete(cleanupState CleanupOperationState) FinalizeDeleteDecision {
	decision, terminal := cleanupOperationProgressDecision(cleanupState, cleanupOperationMessages{
		started:     "upload staging cleanup operation is running",
		unsupported: "upload staging cleanup operation entered an unsupported state",
	})
	if terminal {
		return decision
	}
	return removeFinalizerDecision()
}

func finalizeBackendArtifactDelete(
	cleanupState CleanupOperationState,
	garbageCollectionState GarbageCollectionState,
) FinalizeDeleteDecision {
	decision, terminal := cleanupOperationProgressDecision(cleanupState, cleanupOperationMessages{
		started:     "artifact cleanup operation is running",
		unsupported: "artifact cleanup operation entered an unsupported state",
	})
	if terminal {
		return decision
	}
	return garbageCollectionProgressDecision(garbageCollectionState)
}

func cleanupOperationProgressDecision(state CleanupOperationState, messages cleanupOperationMessages) (FinalizeDeleteDecision, bool) {
	switch state {
	case CleanupOperationStateMissing:
		return runCleanupDecision(messages.started), true
	case CleanupOperationStateComplete:
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

func runCleanupDecision(message string) FinalizeDeleteDecision {
	return FinalizeDeleteDecision{
		RunCleanup:    true,
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
