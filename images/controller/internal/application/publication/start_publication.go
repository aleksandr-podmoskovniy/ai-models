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

package publication

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
	SourceType   modelsv1alpha1.ModelSourceType
	Upload       *modelsv1alpha1.UploadModelSource
	RuntimeHints *modelsv1alpha1.ModelRuntimeHints
}

func StartPublication(input StartPublicationInput) (ExecutionMode, error) {
	switch input.SourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace, modelsv1alpha1.ModelSourceTypeHTTP:
		return ExecutionModeSourceWorker, nil
	case modelsv1alpha1.ModelSourceTypeUpload:
		if input.Upload == nil {
			return "", fmt.Errorf("upload source must not be empty")
		}
		if input.Upload.ExpectedFormat != modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory {
			return "", fmt.Errorf(
				"upload expectedFormat %q is not implemented on the current ModelPack publication adapter; use HuggingFaceDirectory until direct ModelKit ingest is delivered",
				input.Upload.ExpectedFormat,
			)
		}
		if input.RuntimeHints == nil || strings.TrimSpace(input.RuntimeHints.Task) == "" {
			return "", fmt.Errorf(
				"upload source currently requires spec.runtimeHints.task for publication through the current ModelPack adapter",
			)
		}
		return ExecutionModeUpload, nil
	default:
		return "", fmt.Errorf("publication operation does not support source type %q", input.SourceType)
	}
}
