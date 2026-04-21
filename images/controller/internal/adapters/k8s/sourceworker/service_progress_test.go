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

package sourceworker

import (
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestDirectUploadProgress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		state       modelpackports.DirectUploadState
		wantReason  modelsv1alpha1.ModelConditionReason
		wantMessage string
	}{
		{
			name: "starting layer",
			state: modelpackports.DirectUploadState{
				Phase: modelpackports.DirectUploadStatePhaseRunning,
				Stage: modelpackports.DirectUploadStateStageStarting,
				CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
					UploadedSizeBytes: 0,
					TotalSizeBytes:    256,
				},
			},
			wantReason:  modelsv1alpha1.ModelConditionReasonPublicationStarted,
			wantMessage: "controller started model artifact upload into the internal registry: 0/256 bytes uploaded",
		},
		{
			name: "resumed layer",
			state: modelpackports.DirectUploadState{
				Phase: modelpackports.DirectUploadStatePhaseRunning,
				Stage: modelpackports.DirectUploadStateStageResumed,
				CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
					UploadedSizeBytes: 128,
					TotalSizeBytes:    256,
				},
			},
			wantReason:  modelsv1alpha1.ModelConditionReasonPublicationResumed,
			wantMessage: "controller resumed model artifact upload into the internal registry: 128/256 bytes uploaded",
		},
		{
			name: "sealing layer",
			state: modelpackports.DirectUploadState{
				Phase: modelpackports.DirectUploadStatePhaseRunning,
				Stage: modelpackports.DirectUploadStateStageSealing,
				CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
					UploadedSizeBytes: 256,
					TotalSizeBytes:    256,
				},
			},
			wantReason:  modelsv1alpha1.ModelConditionReasonPublicationSealing,
			wantMessage: "controller is sealing the current model artifact layer in the internal registry after 256/256 uploaded bytes",
		},
		{
			name: "committed layers",
			state: modelpackports.DirectUploadState{
				Phase: modelpackports.DirectUploadStatePhaseRunning,
				Stage: modelpackports.DirectUploadStateStageCommitted,
				CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{
					{Key: "a"},
					{Key: "b"},
				},
			},
			wantReason:  modelsv1alpha1.ModelConditionReasonPublicationCommitted,
			wantMessage: "controller is publishing the model artifact: 2 layer(s) already committed into the internal registry",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotReason, gotMessage := directUploadProgress(tc.state)
			if gotReason != tc.wantReason {
				t.Fatalf("directUploadProgress() reason = %q, want %q", gotReason, tc.wantReason)
			}
			if gotMessage != tc.wantMessage {
				t.Fatalf("directUploadProgress() message = %q, want %q", gotMessage, tc.wantMessage)
			}
		})
	}
}

func TestDirectUploadProgressReturnsEmptyOutsideRunning(t *testing.T) {
	t.Parallel()

	reason, message := directUploadProgress(modelpackports.DirectUploadState{
		Phase: modelpackports.DirectUploadStatePhaseCompleted,
	})
	if reason != "" {
		t.Fatalf("unexpected reason %q", reason)
	}
	if strings.TrimSpace(message) != "" {
		t.Fatalf("unexpected message %q", message)
	}
}
