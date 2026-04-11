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
	"errors"
	"reflect"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func TestFinalizeDelete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  FinalizeDeleteInput
		assert func(t *testing.T, got FinalizeDeleteDecision)
	}{
		{
			name:  "missing finalizer is noop",
			input: FinalizeDeleteInput{},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if got.RemoveFinalizer || got.CreateJob || got.UpdateStatus || got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "invalid handle fails closed",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleErr:    errors.New("broken"),
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonCleanupFailed || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "missing handle removes finalizer",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.RemoveFinalizer || got.UpdateStatus {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "unsupported kind is blocked",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.Kind("Other"),
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonCleanupBlocked || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "missing cleanup job creates job",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindBackendArtifact,
				JobState:     CleanupJobStateMissing,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.CreateJob || !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonCleanupPending || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "running cleanup job requeues",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindBackendArtifact,
				JobState:     CleanupJobStateRunning,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if got.CreateJob || !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonCleanupPending || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "failed cleanup job fails closed",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindBackendArtifact,
				JobState:     CleanupJobStateFailed,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonCleanupFailed || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "completed cleanup job requests garbage collection",
			input: FinalizeDeleteInput{
				HasFinalizer:           true,
				HandleFound:            true,
				HandleKind:             cleanuphandle.KindBackendArtifact,
				JobState:               CleanupJobStateComplete,
				GarbageCollectionState: GarbageCollectionStateMissing,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.EnsureGarbageCollectionRequest || !got.UpdateStatus || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "requested garbage collection requeues",
			input: FinalizeDeleteInput{
				HasFinalizer:           true,
				HandleFound:            true,
				HandleKind:             cleanuphandle.KindBackendArtifact,
				JobState:               CleanupJobStateComplete,
				GarbageCollectionState: GarbageCollectionStateRequested,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if got.EnsureGarbageCollectionRequest || !got.UpdateStatus || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "completed cleanup and garbage collection removes finalizer",
			input: FinalizeDeleteInput{
				HasFinalizer:           true,
				HandleFound:            true,
				HandleKind:             cleanuphandle.KindBackendArtifact,
				JobState:               CleanupJobStateComplete,
				GarbageCollectionState: GarbageCollectionStateComplete,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.RemoveFinalizer || !got.DeleteGarbageCollectionRequest || got.UpdateStatus {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "unsupported job state fails closed",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindBackendArtifact,
				JobState:     CleanupJobState("Unknown"),
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonCleanupFailed || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FinalizeDelete(tc.input)
			tc.assert(t, got)
		})
	}
}

func TestFinalizeDeleteUploadStagingLifecycle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  FinalizeDeleteInput
		assert func(t *testing.T, got FinalizeDeleteDecision)
	}{
		{
			name: "upload staging missing job creates cleanup job",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindUploadStaging,
				JobState:     CleanupJobStateMissing,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.CreateJob || !got.UpdateStatus || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload staging running job requeues",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindUploadStaging,
				JobState:     CleanupJobStateRunning,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if got.CreateJob || !got.UpdateStatus || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload staging failed job fails closed",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindUploadStaging,
				JobState:     CleanupJobStateFailed,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonCleanupFailed || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload staging completed job removes finalizer",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindUploadStaging,
				JobState:     CleanupJobStateComplete,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.RemoveFinalizer || got.UpdateStatus || got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload staging unknown job state fails closed",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindUploadStaging,
				JobState:     CleanupJobState("Unknown"),
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonCleanupFailed || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FinalizeDelete(tc.input)
			tc.assert(t, got)
		})
	}
}

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
				StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
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
				StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
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
				StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupFailed,
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
				StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupFailed,
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
			name:  "missing gc requests registry cleanup",
			state: GarbageCollectionStateMissing,
			want: FinalizeDeleteDecision{
				EnsureGarbageCollectionRequest: true,
				UpdateStatus:                   true,
				StatusReason:                   modelsv1alpha1.ModelConditionReasonCleanupPending,
				StatusMessage:                  "registry garbage collection requested",
				Requeue:                        true,
			},
		},
		{
			name:  "requested gc keeps pending status",
			state: GarbageCollectionStateRequested,
			want: FinalizeDeleteDecision{
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupPending,
				StatusMessage: "registry garbage collection is still running",
				Requeue:       true,
			},
		},
		{
			name:  "completed gc removes finalizer and request",
			state: GarbageCollectionStateComplete,
			want: FinalizeDeleteDecision{
				DeleteGarbageCollectionRequest: true,
				RemoveFinalizer:                true,
			},
		},
		{
			name:  "unsupported gc state fails closed",
			state: GarbageCollectionState("Other"),
			want: FinalizeDeleteDecision{
				UpdateStatus:  true,
				StatusReason:  modelsv1alpha1.ModelConditionReasonCleanupFailed,
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
