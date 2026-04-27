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

type SummaryInput struct {
	ConfigPayload          []byte
	WeightBytes            int64
	LargestWeightFileBytes int64
	WeightFileCount        int64
	Task                   string
	TaskHint               string
}

func Resolve(input Input) (publicationdata.ResolvedProfile, error) {
	if strings.TrimSpace(input.CheckpointDir) == "" {
		return publicationdata.ResolvedProfile{}, errors.New("checkpoint directory must not be empty")
	}

	config, err := loadConfig(filepath.Join(input.CheckpointDir, "config.json"))
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}

	weights, err := totalWeightStats(input.CheckpointDir)
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}

	return resolveSummary(config, weights, input.Task, input.TaskHint)
}

func ResolveSummary(input SummaryInput) (publicationdata.ResolvedProfile, error) {
	if len(input.ConfigPayload) == 0 {
		return publicationdata.ResolvedProfile{}, errors.New("checkpoint config payload must not be empty")
	}
	if input.WeightBytes <= 0 {
		return publicationdata.ResolvedProfile{}, errors.New("safetensors weight bytes must be positive")
	}

	config, err := decodeConfig(input.ConfigPayload)
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}

	weights := weightStats{
		totalBytes:       input.WeightBytes,
		largestFileBytes: input.LargestWeightFileBytes,
		fileCount:        input.WeightFileCount,
	}
	return resolveSummary(config, weights, input.Task, input.TaskHint)
}

func resolveSummary(
	config map[string]any,
	weights weightStats,
	task string,
	taskHint string,
) (publicationdata.ResolvedProfile, error) {
	architecture := resolveArchitecture(config)
	family, familyConfidence := resolveFamily(config, architecture)
	task, taskConfidence := resolveTask(config, architecture, task, taskHint)
	contextWindow := detectContextWindow(config)
	quantization := detectQuantization(config)
	precision := detectPrecision(config, quantization)
	parameterCount, parameterConfidence := resolveParameterCount(config, weights.totalBytes, precision, quantization)
	endpoints := []string(nil)
	if taskConfidence.ReliablePublicFact() {
		endpoints = profilecommon.EndpointTypes(task)
	}
	footprint := publicationdata.ProfileFootprint{
		WeightsBytes:           weights.totalBytes,
		LargestWeightFileBytes: weights.largestFileBytes,
		ShardCount:             weights.fileCount,
		EstimatedWorkingSetGiB: profilecommon.EstimatedWorkingSetGiB(
			weights.totalBytes,
			parameterCount,
			precision,
			quantization,
		),
	}

	resolved := publicationdata.ResolvedProfile{
		Task:                          task,
		TaskConfidence:                taskConfidence,
		Family:                        family,
		FamilyConfidence:              familyConfidence,
		Architecture:                  architecture,
		ArchitectureConfidence:        confidenceIfSet(architecture, publicationdata.ProfileConfidenceExact),
		Format:                        "Safetensors",
		ParameterCount:                parameterCount,
		ParameterCountConfidence:      parameterConfidence,
		Quantization:                  quantization,
		QuantizationConfidence:        confidenceIfSet(quantization, publicationdata.ProfileConfidenceExact),
		ContextWindowTokens:           contextWindow,
		ContextWindowTokensConfidence: confidenceIfPositive(contextWindow, publicationdata.ProfileConfidenceExact),
		SupportedEndpointTypes:        endpoints,
		Footprint:                     footprint,
	}

	return resolved, nil
}

func confidenceIfSet(value string, confidence publicationdata.ProfileConfidence) publicationdata.ProfileConfidence {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return confidence
}

func confidenceIfPositive(value int64, confidence publicationdata.ProfileConfidence) publicationdata.ProfileConfidence {
	if value <= 0 {
		return ""
	}
	return confidence
}
