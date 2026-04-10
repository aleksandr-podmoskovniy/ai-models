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

package publishobserve

import (
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
)

type CatalogStatusReconcileInput struct {
	Deleting           bool
	Source             modelsv1alpha1.ModelSourceSpec
	RuntimeHints       *modelsv1alpha1.ModelRuntimeHints
	UploadStagePresent bool
	Current            modelsv1alpha1.ModelStatus
	Generation         int64
	HasCleanupHandle   bool
}

type CatalogStatusReconcileDecision struct {
	Skip       bool
	SourceType modelsv1alpha1.ModelSourceType
	Mode       publicationapp.ExecutionMode
}

func DecideCatalogStatusReconcile(
	input CatalogStatusReconcileInput,
) (CatalogStatusReconcileDecision, error) {
	if input.Deleting {
		return CatalogStatusReconcileDecision{Skip: true}, nil
	}

	sourceType, err := input.Source.DetectType()
	if err != nil {
		return CatalogStatusReconcileDecision{}, err
	}

	if shouldSkipCatalogStatusReconcile(input.Current, input.Generation, input.HasCleanupHandle) {
		return CatalogStatusReconcileDecision{
			Skip:       true,
			SourceType: sourceType,
		}, nil
	}

	mode, err := publicationapp.StartPublication(publicationapp.StartPublicationInput{
		Source:             input.Source,
		RuntimeHints:       input.RuntimeHints,
		UploadStagePresent: input.UploadStagePresent,
	})
	if err != nil {
		return CatalogStatusReconcileDecision{}, err
	}

	return CatalogStatusReconcileDecision{
		SourceType: sourceType,
		Mode:       mode,
	}, nil
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
