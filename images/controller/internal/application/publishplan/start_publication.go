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

package publishplan

import (
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

type ExecutionMode string

const (
	ExecutionModeSourceWorker ExecutionMode = "SourceWorker"
	ExecutionModeUpload       ExecutionMode = "UploadSession"
)

type StartPublicationInput struct {
	Source       modelsv1alpha1.ModelSourceSpec
	RuntimeHints *modelsv1alpha1.ModelRuntimeHints
}

func StartPublication(input StartPublicationInput) (ExecutionMode, error) {
	sourceType, err := input.Source.DetectType()
	if err != nil {
		return "", err
	}

	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace, modelsv1alpha1.ModelSourceTypeHTTP:
		return ExecutionModeSourceWorker, nil
	case modelsv1alpha1.ModelSourceTypeUpload:
		if input.Source.Upload == nil {
			return "", fmt.Errorf("upload source must not be empty")
		}
		if input.RuntimeHints == nil || strings.TrimSpace(input.RuntimeHints.Task) == "" {
			return "", fmt.Errorf("upload source currently requires spec.runtimeHints.task")
		}
		return ExecutionModeUpload, nil
	default:
		return "", fmt.Errorf("publication operation does not support source type %q", sourceType)
	}
}
