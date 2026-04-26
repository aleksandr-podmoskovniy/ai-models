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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func TestFinalizeDeleteUploadStagingLifecycle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  FinalizeDeleteInput
		assert func(t *testing.T, got FinalizeDeleteDecision)
	}{
		{
			name: "upload staging missing cleanup runs operation",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindUploadStaging,
				CleanupState: CleanupOperationStateMissing,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.RunCleanup || !got.UpdateStatus || !got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload staging completed cleanup removes finalizer",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindUploadStaging,
				CleanupState: CleanupOperationStateComplete,
			},
			assert: func(t *testing.T, got FinalizeDeleteDecision) {
				t.Helper()
				if !got.RemoveFinalizer || got.UpdateStatus || got.Requeue {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload staging unknown cleanup state fails closed",
			input: FinalizeDeleteInput{
				HasFinalizer: true,
				HandleFound:  true,
				HandleKind:   cleanuphandle.KindUploadStaging,
				CleanupState: CleanupOperationState("Unknown"),
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
