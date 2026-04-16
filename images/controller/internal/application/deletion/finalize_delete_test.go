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
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonFailed || !got.Requeue {
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
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonFailed || !got.Requeue {
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
				if !got.CreateJob || !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonPending || !got.Requeue {
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
				if got.CreateJob || !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonPending || !got.Requeue {
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
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonFailed || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "completed cleanup job enqueues garbage collection and removes finalizer",
			input: FinalizeDeleteInput{
				HasFinalizer:           true,
				HandleFound:            true,
				HandleKind:             cleanuphandle.KindBackendArtifact,
				JobState:               CleanupJobStateComplete,
				GarbageCollectionState: GarbageCollectionStateMissing,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.EnsureGarbageCollectionRequest || !got.RemoveFinalizer || got.UpdateStatus || got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "queued garbage collection removes finalizer",
			input: FinalizeDeleteInput{
				HasFinalizer:           true,
				HandleFound:            true,
				HandleKind:             cleanuphandle.KindBackendArtifact,
				JobState:               CleanupJobStateComplete,
				GarbageCollectionState: GarbageCollectionStateQueued,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if got.EnsureGarbageCollectionRequest || !got.RemoveFinalizer || got.UpdateStatus || got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "requested garbage collection removes finalizer",
			input: FinalizeDeleteInput{
				HasFinalizer:           true,
				HandleFound:            true,
				HandleKind:             cleanuphandle.KindBackendArtifact,
				JobState:               CleanupJobStateComplete,
				GarbageCollectionState: GarbageCollectionStateRequested,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if got.EnsureGarbageCollectionRequest || !got.RemoveFinalizer || got.UpdateStatus || got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "completed cleanup and garbage collection still removes finalizer",
			input: FinalizeDeleteInput{
				HasFinalizer:           true,
				HandleFound:            true,
				HandleKind:             cleanuphandle.KindBackendArtifact,
				JobState:               CleanupJobStateComplete,
				GarbageCollectionState: GarbageCollectionStateComplete,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.RemoveFinalizer || got.DeleteGarbageCollectionRequest || got.UpdateStatus {
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
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonFailed || !got.Requeue {
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
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonFailed || !got.Requeue {
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
				if !got.UpdateStatus || got.StatusReason != modelsv1alpha1.ModelConditionReasonFailed || !got.Requeue {
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
