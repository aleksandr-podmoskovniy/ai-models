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

package catalogstatus

import (
	"fmt"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

type runtimeMode string

const (
	runtimeModeSourceWorker runtimeMode = "SourceWorker"
	runtimeModeUpload       runtimeMode = "UploadSession"
)

type reconcileDecision struct {
	Skip       bool
	SourceType modelsv1alpha1.ModelSourceType
	Mode       runtimeMode
}

func decideCatalogStatusReconcile(
	deleting bool,
	source modelsv1alpha1.ModelSourceSpec,
	uploadStagePresent bool,
	current modelsv1alpha1.ModelStatus,
	generation int64,
	hasCleanupHandle bool,
) (reconcileDecision, error) {
	if deleting {
		return reconcileDecision{Skip: true}, nil
	}

	sourceType, err := source.DetectType()
	if err != nil {
		return reconcileDecision{}, err
	}
	if shouldSkipCatalogStatusReconcile(current, generation, hasCleanupHandle) {
		return reconcileDecision{Skip: true, SourceType: sourceType}, nil
	}

	mode, err := sourceRuntimeMode(source, uploadStagePresent)
	if err != nil {
		return reconcileDecision{}, err
	}
	if sourceType == modelsv1alpha1.ModelSourceTypeUpload &&
		shouldKeepUploadSourceWorker(current, generation, hasCleanupHandle) {
		mode = runtimeModeSourceWorker
	}

	return reconcileDecision{
		SourceType: sourceType,
		Mode:       mode,
	}, nil
}

func sourceRuntimeMode(source modelsv1alpha1.ModelSourceSpec, uploadStagePresent bool) (runtimeMode, error) {
	sourceType, err := source.DetectType()
	if err != nil {
		return "", err
	}

	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace:
		return runtimeModeSourceWorker, nil
	case modelsv1alpha1.ModelSourceTypeUpload:
		if source.Upload == nil {
			return "", fmt.Errorf("upload source must not be empty")
		}
		if uploadStagePresent {
			return runtimeModeSourceWorker, nil
		}
		return runtimeModeUpload, nil
	default:
		return "", fmt.Errorf("publication operation does not support source type %q", sourceType)
	}
}

func shouldSkipCatalogStatusReconcile(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	hasCleanupHandle bool,
) bool {
	if current.ObservedGeneration != generation {
		return false
	}

	switch current.Phase {
	case modelsv1alpha1.ModelPhaseReady:
		return hasCleanupHandle
	case modelsv1alpha1.ModelPhaseFailed:
		return true
	default:
		return false
	}
}

func shouldKeepUploadSourceWorker(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	hasCleanupHandle bool,
) bool {
	return hasCleanupHandle &&
		current.ObservedGeneration == generation &&
		current.Phase == modelsv1alpha1.ModelPhasePublishing
}
