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
		name         string
		state        modelpackports.DirectUploadState
		wantReason   modelsv1alpha1.ModelConditionReason
		wantProgress string
		wantMessage  string
	}{
		{
			name: "starting layer",
			state: modelpackports.DirectUploadState{
				Phase:             modelpackports.DirectUploadStatePhaseRunning,
				Stage:             modelpackports.DirectUploadStateStageStarting,
				PlannedLayerCount: 4,
				PlannedSizeBytes:  1024,
				CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
					UploadedSizeBytes: 0,
					TotalSizeBytes:    256,
				},
			},
			wantReason:   modelsv1alpha1.ModelConditionReasonPublicationStarted,
			wantProgress: "0%",
			wantMessage:  "controller started model artifact upload into the internal registry: 0/1024 bytes uploaded",
		},
		{
			name: "resumed layer",
			state: modelpackports.DirectUploadState{
				Phase:             modelpackports.DirectUploadStatePhaseRunning,
				Stage:             modelpackports.DirectUploadStateStageResumed,
				PlannedLayerCount: 4,
				PlannedSizeBytes:  1024,
				CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{
					{Key: "a", SizeBytes: 256},
				},
				CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
					UploadedSizeBytes: 128,
					TotalSizeBytes:    256,
				},
			},
			wantReason:   modelsv1alpha1.ModelConditionReasonPublicationResumed,
			wantProgress: "37%",
			wantMessage:  "controller resumed model artifact upload into the internal registry: 384/1024 bytes uploaded",
		},
		{
			name: "sealing layer",
			state: modelpackports.DirectUploadState{
				Phase:             modelpackports.DirectUploadStatePhaseRunning,
				Stage:             modelpackports.DirectUploadStateStageSealing,
				PlannedLayerCount: 4,
				PlannedSizeBytes:  1024,
				CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{
					{Key: "a", SizeBytes: 512},
				},
				CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
					UploadedSizeBytes: 256,
					TotalSizeBytes:    256,
				},
			},
			wantReason:   modelsv1alpha1.ModelConditionReasonPublicationSealing,
			wantProgress: "75%",
			wantMessage:  "controller is sealing the current model artifact layer in the internal registry after 768/1024 uploaded bytes",
		},
		{
			name: "committed layers",
			state: modelpackports.DirectUploadState{
				Phase:             modelpackports.DirectUploadStatePhaseRunning,
				Stage:             modelpackports.DirectUploadStateStageCommitted,
				PlannedLayerCount: 2,
				PlannedSizeBytes:  1024,
				CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{
					{Key: "a", SizeBytes: 512},
					{Key: "b", SizeBytes: 512},
				},
			},
			wantReason:   modelsv1alpha1.ModelConditionReasonPublicationCommitted,
			wantProgress: "99%",
			wantMessage:  "controller is publishing the model artifact: 2/2 layer(s) already committed into the internal registry",
		},
		{
			name: "legacy state without planned totals keeps per-layer message",
			state: modelpackports.DirectUploadState{
				Phase: modelpackports.DirectUploadStatePhaseRunning,
				Stage: modelpackports.DirectUploadStateStageUploading,
				CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
					UploadedSizeBytes: 128,
					TotalSizeBytes:    256,
				},
			},
			wantReason:   modelsv1alpha1.ModelConditionReasonPublicationUploading,
			wantProgress: "",
			wantMessage:  "controller is publishing the model artifact: 128/256 bytes uploaded into the internal registry",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := directUploadProgress(tc.state)
			if got.Reason != tc.wantReason {
				t.Fatalf("directUploadProgress() reason = %q, want %q", got.Reason, tc.wantReason)
			}
			if got.Progress != tc.wantProgress {
				t.Fatalf("directUploadProgress() progress = %q, want %q", got.Progress, tc.wantProgress)
			}
			if got.Message != tc.wantMessage {
				t.Fatalf("directUploadProgress() message = %q, want %q", got.Message, tc.wantMessage)
			}
		})
	}
}

func TestDirectUploadProgressReturnsEmptyOutsideRunning(t *testing.T) {
	t.Parallel()

	progress := directUploadProgress(modelpackports.DirectUploadState{
		Phase: modelpackports.DirectUploadStatePhaseCompleted,
	})
	if progress.Reason != "" {
		t.Fatalf("unexpected reason %q", progress.Reason)
	}
	if strings.TrimSpace(progress.Progress) != "" {
		t.Fatalf("unexpected progress %q", progress.Progress)
	}
	if strings.TrimSpace(progress.Message) != "" {
		t.Fatalf("unexpected message %q", progress.Message)
	}
}
