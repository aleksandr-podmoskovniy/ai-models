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
	"fmt"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type directUploadProgressStatus struct {
	Reason   modelsv1alpha1.ModelConditionReason
	Progress string
	Message  string
}

func directUploadProgress(state modelpackports.DirectUploadState) directUploadProgressStatus {
	if state.Phase != modelpackports.DirectUploadStatePhaseRunning {
		return directUploadProgressStatus{}
	}

	progress := publicationProgressPercent(state)
	if state.CurrentLayer != nil {
		uploadedSizeBytes, totalSizeBytes := progressMessageSizes(state)
		switch state.Stage {
		case modelpackports.DirectUploadStateStageStarting:
			return directUploadProgressStatus{
				Reason:   modelsv1alpha1.ModelConditionReasonPublicationStarted,
				Progress: progress,
				Message: fmt.Sprintf(
					"controller started model artifact upload into the internal registry: %d/%d bytes uploaded",
					uploadedSizeBytes,
					totalSizeBytes,
				),
			}
		case modelpackports.DirectUploadStateStageResumed:
			return directUploadProgressStatus{
				Reason:   modelsv1alpha1.ModelConditionReasonPublicationResumed,
				Progress: progress,
				Message: fmt.Sprintf(
					"controller resumed model artifact upload into the internal registry: %d/%d bytes uploaded",
					uploadedSizeBytes,
					totalSizeBytes,
				),
			}
		case modelpackports.DirectUploadStateStageSealing:
			return directUploadProgressStatus{
				Reason:   modelsv1alpha1.ModelConditionReasonPublicationSealing,
				Progress: progress,
				Message: fmt.Sprintf(
					"controller is verifying and sealing the current model artifact layer in the internal registry after %d/%d uploaded bytes",
					uploadedSizeBytes,
					totalSizeBytes,
				),
			}
		default:
			return directUploadProgressStatus{
				Reason:   modelsv1alpha1.ModelConditionReasonPublicationUploading,
				Progress: progress,
				Message: fmt.Sprintf(
					"controller is publishing the model artifact: %d/%d bytes uploaded into the internal registry",
					uploadedSizeBytes,
					totalSizeBytes,
				),
			}
		}
	}

	if len(state.CompletedLayers) > 0 {
		if state.PlannedLayerCount > 0 {
			return directUploadProgressStatus{
				Reason:   modelsv1alpha1.ModelConditionReasonPublicationCommitted,
				Progress: progress,
				Message: fmt.Sprintf(
					"controller is publishing the model artifact: %d/%d layer(s) already committed into the internal registry",
					len(state.CompletedLayers),
					state.PlannedLayerCount,
				),
			}
		}
		return directUploadProgressStatus{
			Reason:   modelsv1alpha1.ModelConditionReasonPublicationCommitted,
			Progress: progress,
			Message: fmt.Sprintf(
				"controller is publishing the model artifact: %d layer(s) already committed into the internal registry",
				len(state.CompletedLayers),
			),
		}
	}

	return directUploadProgressStatus{
		Reason:   modelsv1alpha1.ModelConditionReasonPending,
		Progress: progress,
	}
}

func publicationProgressPercent(state modelpackports.DirectUploadState) string {
	if state.PlannedSizeBytes <= 0 {
		return ""
	}

	uploadedSizeBytes := directUploadUploadedSizeBytes(state)
	switch {
	case uploadedSizeBytes <= 0:
		return "0%"
	case uploadedSizeBytes >= state.PlannedSizeBytes:
		return "99%"
	default:
		progress := int((uploadedSizeBytes * 100) / state.PlannedSizeBytes)
		if progress >= 100 {
			progress = 99
		}
		return fmt.Sprintf("%d%%", progress)
	}
}

func progressMessageSizes(state modelpackports.DirectUploadState) (int64, int64) {
	uploadedSizeBytes := directUploadUploadedSizeBytes(state)
	if state.PlannedSizeBytes > 0 {
		return uploadedSizeBytes, state.PlannedSizeBytes
	}
	if state.CurrentLayer == nil {
		return uploadedSizeBytes, uploadedSizeBytes
	}
	return state.CurrentLayer.UploadedSizeBytes, state.CurrentLayer.TotalSizeBytes
}

func directUploadUploadedSizeBytes(state modelpackports.DirectUploadState) int64 {
	var uploadedSizeBytes int64
	for _, layer := range state.CompletedLayers {
		if layer.SizeBytes > 0 {
			uploadedSizeBytes += layer.SizeBytes
		}
	}
	if state.CurrentLayer != nil && state.CurrentLayer.UploadedSizeBytes > 0 {
		uploadedSizeBytes += state.CurrentLayer.UploadedSizeBytes
	}
	return uploadedSizeBytes
}
