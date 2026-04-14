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

package publishworker

import (
	"context"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	ggufprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/gguf"
	safetensorsprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/safetensors"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type sourceProfileInput struct {
	Task           string
	TaskHint       string
	RuntimeEngines []string
	Provenance     sourceProfileProvenance
}

type sourceProfileProvenance struct {
	License      string
	SourceRepoID string
}

func resolveAndPublish(
	ctx context.Context,
	options Options,
	checkpointDir string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	input sourceProfileInput,
	description string,
) (publicationdata.ResolvedProfile, modelpackports.PublishResult, error) {
	resolvedProfile, err := resolveProfile(checkpointDir, inputFormat, input)
	if err != nil {
		return publicationdata.ResolvedProfile{}, modelpackports.PublishResult{}, err
	}
	resolvedProfile = attachResolvedProfileProvenance(resolvedProfile, input.Provenance)

	publishResult, err := options.ModelPackPublisher.Publish(ctx, modelpackports.PublishInput{
		ModelDir:    checkpointDir,
		ArtifactURI: options.ArtifactURI,
		Description: description,
	}, options.RegistryAuth)
	if err != nil {
		return publicationdata.ResolvedProfile{}, modelpackports.PublishResult{}, err
	}

	return resolvedProfile, publishResult, nil
}

func resolveProfile(
	checkpointDir string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	input sourceProfileInput,
) (publicationdata.ResolvedProfile, error) {
	switch inputFormat {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return safetensorsprofile.Resolve(safetensorsprofile.Input{
			CheckpointDir:  checkpointDir,
			Task:           input.Task,
			TaskHint:       input.TaskHint,
			RuntimeEngines: input.RuntimeEngines,
		})
	case modelsv1alpha1.ModelInputFormatGGUF:
		return ggufprofile.Resolve(ggufprofile.Input{
			ModelDir:       checkpointDir,
			Task:           input.Task,
			RuntimeEngines: input.RuntimeEngines,
		})
	default:
		return publicationdata.ResolvedProfile{}, fmt.Errorf("unsupported model input format %q", inputFormat)
	}
}

func attachResolvedProfileProvenance(
	resolved publicationdata.ResolvedProfile,
	provenance sourceProfileProvenance,
) publicationdata.ResolvedProfile {
	resolved.License = strings.TrimSpace(provenance.License)
	resolved.SourceRepoID = strings.TrimSpace(provenance.SourceRepoID)
	return resolved
}
