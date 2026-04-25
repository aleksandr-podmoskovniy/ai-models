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

func TestCleanupJobProgressDecision(t *testing.T) {
	t.Parallel()

	messages := cleanupJobMessages{
		created:     "job created",
		running:     "job running",
		failed:      "job failed",
		unsupported: "job unsupported",
	}

	tests := []struct {
		name        string
		jobState    CleanupJobState
		want        FinalizeDeleteDecision
		wantHandled bool
	}{
		{
			name:     "missing job creates pending decision",
			jobState: CleanupJobStateMissing,
			want: FinalizeDeleteDecision{
				CreateJob:     true,
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonPending,
				StatusMessage: "job created",
				Requeue:       true,
			},
			wantHandled: true,
		},
		{
			name:     "running job keeps pending decision",
			jobState: CleanupJobStateRunning,
			want: FinalizeDeleteDecision{
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonPending,
				StatusMessage: "job running",
				Requeue:       true,
			},
			wantHandled: true,
		},
		{
			name:     "failed job fails closed",
			jobState: CleanupJobStateFailed,
			want: FinalizeDeleteDecision{
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonFailed,
				StatusMessage: "job failed",
				Requeue:       true,
			},
			wantHandled: true,
		},
		{
			name:        "completed job delegates to next phase",
			jobState:    CleanupJobStateComplete,
			want:        FinalizeDeleteDecision{},
			wantHandled: false,
		},
		{
			name:     "unsupported job state fails closed",
			jobState: CleanupJobState("Unknown"),
			want: FinalizeDeleteDecision{
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonFailed,
				StatusMessage: "job unsupported",
				Requeue:       true,
			},
			wantHandled: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, handled := cleanupJobProgressDecision(tt.jobState, messages)
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
