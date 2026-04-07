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
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func shouldIgnoreObject(object client.Object, sourceType modelsv1alpha1.ModelSourceType) bool {
	if !object.GetDeletionTimestamp().IsZero() {
		return true
	}
	return !supportsSourceType(sourceType)
}

func supportsSourceType(sourceType modelsv1alpha1.ModelSourceType) bool {
	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace,
		modelsv1alpha1.ModelSourceTypeUpload,
		modelsv1alpha1.ModelSourceTypeHTTP:
		return true
	default:
		return false
	}
}

func shouldSkipReconcile(
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
