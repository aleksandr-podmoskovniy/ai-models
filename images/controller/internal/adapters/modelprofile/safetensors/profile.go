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

package safetensors

import (
	"errors"
	"path/filepath"
	"strings"

	profilecommon "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/common"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type Input struct {
	CheckpointDir string
	Task          string
	TaskHint      string
}

func Resolve(input Input) (publicationdata.ResolvedProfile, error) {
	if strings.TrimSpace(input.CheckpointDir) == "" {
		return publicationdata.ResolvedProfile{}, errors.New("checkpoint directory must not be empty")
	}

	config, err := loadConfig(filepath.Join(input.CheckpointDir, "config.json"))
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}

	weightBytes, err := totalWeightBytes(input.CheckpointDir)
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}

	architecture := resolveArchitecture(config)
	family := resolveFamily(config, architecture)
	task := resolveTask(config, architecture, input.Task, input.TaskHint)
	contextWindow := detectContextWindow(config)
	quantization := detectQuantization(config)
	precision := detectPrecision(config, quantization)
	parameterCount := estimateParameterCount(config)
	if parameterCount <= 0 {
		parameterCount = profilecommon.EstimateParameterCountFromBytes(weightBytes, precision, quantization)
	}

	minimumLaunch := profilecommon.MinimumGPULaunch(
		profilecommon.GPUWorkingSetGiB(weightBytes, parameterCount, precision, quantization),
	)

	resolved := publicationdata.ResolvedProfile{
		Task:                   task,
		Framework:              "transformers",
		Family:                 family,
		Architecture:           architecture,
		Format:                 "Safetensors",
		ParameterCount:         parameterCount,
		Quantization:           quantization,
		ContextWindowTokens:    contextWindow,
		SupportedEndpointTypes: profilecommon.EndpointTypes(task),
		CompatiblePrecisions:   compatiblePrecisions(precision),
		MinimumLaunch:          minimumLaunch,
	}
	if minimumLaunch.PlacementType == "GPU" {
		resolved.CompatibleAcceleratorVendors = profilecommon.GPUVendors()
	}

	return resolved, nil
}
