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
	"reflect"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestCleanupOperationProgressDecision(t *testing.T) {
	t.Parallel()

	messages := cleanupOperationMessages{
		started:     "cleanup started",
		unsupported: "cleanup unsupported",
	}

	tests := []struct {
		name        string
		state       CleanupOperationState
		want        FinalizeDeleteDecision
		wantHandled bool
	}{
		{
			name:  "missing cleanup runs operation",
			state: CleanupOperationStateMissing,
			want: FinalizeDeleteDecision{
				RunCleanup:    true,
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonPending,
				StatusMessage: "cleanup started",
				Requeue:       true,
			},
			wantHandled: true,
		},
		{
			name:        "completed cleanup delegates to next phase",
			state:       CleanupOperationStateComplete,
			want:        FinalizeDeleteDecision{},
			wantHandled: false,
		},
		{
			name:  "unsupported cleanup state fails closed",
			state: CleanupOperationState("Unknown"),
			want: FinalizeDeleteDecision{
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonFailed,
				StatusMessage: "cleanup unsupported",
				Requeue:       true,
			},
			wantHandled: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, handled := cleanupOperationProgressDecision(tt.state, messages)
			if handled != tt.wantHandled {
				t.Fatalf("handled = %v, want %v", handled, tt.wantHandled)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("decision = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGarbageCollectionProgressDecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state GarbageCollectionState
		want  FinalizeDeleteDecision
	}{
		{
			name:  "missing gc enqueues registry cleanup and removes finalizer",
			state: GarbageCollectionStateMissing,
			want: FinalizeDeleteDecision{
				EnsureGarbageCollectionRequest: true,
				RemoveFinalizer:                true,
			},
		},
		{
			name:  "queued gc removes finalizer",
			state: GarbageCollectionStateQueued,
			want: FinalizeDeleteDecision{
				RemoveFinalizer: true,
			},
		},
		{
			name:  "requested gc removes finalizer",
			state: GarbageCollectionStateRequested,
			want: FinalizeDeleteDecision{
				RemoveFinalizer: true,
			},
		},
		{
			name:  "completed gc removes finalizer",
			state: GarbageCollectionStateComplete,
			want: FinalizeDeleteDecision{
				RemoveFinalizer: true,
			},
		},
		{
			name:  "unsupported gc state fails closed",
			state: GarbageCollectionState("Other"),
			want: FinalizeDeleteDecision{
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonFailed,
				StatusMessage: "registry garbage collection entered an unsupported state",
				Requeue:       true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := garbageCollectionProgressDecision(tt.state)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("decision = %#v, want %#v", got, tt.want)
			}
		})
	}
}
