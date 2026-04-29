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
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	diffusersprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/diffusers"
	ggufprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/gguf"
	safetensorsprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/safetensors"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func resolveRemoteProfile(
	options Options,
	remote sourcefetch.RemoteResult,
) (*publicationdata.ResolvedProfile, error) {
	if remote.ProfileSummary == nil {
		return nil, nil
	}

	switch remote.InputFormat {
	case modelsv1alpha1.ModelInputFormatDiffusers:
		resolved, err := diffusersprofile.ResolveSummary(diffusersprofile.SummaryInput{
			ModelIndexPayload:      remote.ProfileSummary.ModelIndexPayload,
			WeightBytes:            remote.ProfileSummary.WeightBytes,
			LargestWeightFileBytes: remote.ProfileSummary.LargestWeightFileBytes,
			WeightFileCount:        remote.ProfileSummary.WeightFileCount,
			Task:                   options.Task,
			SourceDeclaredTask:     remote.Fallbacks.SourceDeclaredTask,
			TaskHint:               remote.Fallbacks.TaskHint,
		})
		if err != nil {
			return nil, err
		}
		return &resolved, nil
	case modelsv1alpha1.ModelInputFormatSafetensors:
		resolved, err := safetensorsprofile.ResolveSummary(safetensorsprofile.SummaryInput{
			ConfigPayload:          remote.ProfileSummary.ConfigPayload,
			TokenizerConfigPayload: remote.ProfileSummary.TokenizerConfigPayload,
			WeightBytes:            remote.ProfileSummary.WeightBytes,
			LargestWeightFileBytes: remote.ProfileSummary.LargestWeightFileBytes,
			WeightFileCount:        remote.ProfileSummary.WeightFileCount,
			Task:                   options.Task,
			SourceDeclaredTask:     remote.Fallbacks.SourceDeclaredTask,
			TaskHint:               remote.Fallbacks.TaskHint,
		})
		if err != nil {
			return nil, err
		}
		return &resolved, nil
	case modelsv1alpha1.ModelInputFormatGGUF:
		resolved, err := ggufprofile.ResolveSummary(ggufprofile.SummaryInput{
			ModelFileName:       remote.ProfileSummary.ModelFileName,
			ModelSizeBytes:      remote.ProfileSummary.ModelSizeBytes,
			Task:                options.Task,
			SourceDeclaredTask:  remote.Fallbacks.SourceDeclaredTask,
			TaskHint:            remote.Fallbacks.TaskHint,
			Family:              remote.ProfileSummary.Family,
			Architecture:        remote.ProfileSummary.Architecture,
			ParameterCount:      remote.ProfileSummary.ParameterCount,
			Quantization:        remote.ProfileSummary.Quantization,
			ContextWindowTokens: remote.ProfileSummary.ContextWindowTokens,
		})
		if err != nil {
			return nil, err
		}
		return &resolved, nil
	default:
		return nil, nil
	}
}
