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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	profilecommon "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/common"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type Input struct {
	CheckpointDir  string
	Task           string
	Framework      string
	License        string
	SourceRepoID   string
	RuntimeEngines []string
}

func Resolve(input Input) (publicationdata.ResolvedProfile, error) {
	if strings.TrimSpace(input.CheckpointDir) == "" {
		return publicationdata.ResolvedProfile{}, errors.New("checkpoint directory must not be empty")
	}
	if strings.TrimSpace(input.Task) == "" {
		return publicationdata.ResolvedProfile{}, errors.New("resolved task must not be empty")
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
	family := resolveFamily(config, architecture, input.SourceRepoID)
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
		Task:                strings.TrimSpace(input.Task),
		Framework:           firstNonEmpty(input.Framework, "transformers"),
		Family:              family,
		License:             strings.TrimSpace(input.License),
		Architecture:        architecture,
		Format:              "Safetensors",
		ParameterCount:      parameterCount,
		Quantization:        quantization,
		ContextWindowTokens: contextWindow,
		SourceRepoID:        strings.TrimSpace(input.SourceRepoID),
		SupportedEndpointTypes: profilecommon.EndpointTypes(
			input.Task,
		),
		CompatibleRuntimes:   compatibleRuntimes(input.RuntimeEngines),
		CompatiblePrecisions: compatiblePrecisions(precision),
		MinimumLaunch:        minimumLaunch,
	}
	if minimumLaunch.PlacementType == "GPU" {
		resolved.CompatibleAcceleratorVendors = profilecommon.GPUVendors()
	}

	return resolved, nil
}

func loadConfig(path string) (map[string]any, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint config.json: %w", err)
	}

	var config map[string]any
	if err := json.Unmarshal(payload, &config); err != nil {
		return nil, fmt.Errorf("failed to decode checkpoint config.json: %w", err)
	}
	return config, nil
}

func totalWeightBytes(root string) (int64, error) {
	var total int64

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".safetensors") {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}

	return total, nil
}

func summaryValue(config map[string]any, key string) any {
	if textConfig, _ := config["text_config"].(map[string]any); textConfig != nil {
		if value, found := textConfig[key]; found {
			return value
		}
	}
	return config[key]
}

func stringValue(value any) string {
	typed, _ := value.(string)
	return strings.TrimSpace(typed)
}

func stringSlice(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if itemString, ok := item.(string); ok && strings.TrimSpace(itemString) != "" {
			result = append(result, strings.TrimSpace(itemString))
		}
	}
	return result
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}

func resolveArchitecture(config map[string]any) string {
	architectures := stringSlice(config["architectures"])
	return firstNonEmpty(architectures...)
}

func resolveFamily(config map[string]any, architecture, sourceRepoID string) string {
	if family := strings.TrimSpace(stringValue(config["model_type"])); family != "" {
		return strings.ToLower(family)
	}

	if normalized := normalizeArchitecture(architecture); normalized != "" {
		return strings.ToLower(normalized)
	}

	if sourceRepoID != "" {
		parts := strings.Split(strings.TrimSpace(sourceRepoID), "/")
		if len(parts) > 0 {
			return strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
		}
	}

	return ""
}

func normalizeArchitecture(value string) string {
	normalized := strings.TrimSpace(value)
	for _, suffix := range []string{
		"ForCausalLM",
		"ForConditionalGeneration",
		"ForSequenceClassification",
		"ForTokenClassification",
		"ForQuestionAnswering",
		"Model",
	} {
		normalized = strings.TrimSuffix(normalized, suffix)
	}
	return strings.TrimSpace(normalized)
}

func detectContextWindow(config map[string]any) int64 {
	for _, key := range []string{
		"max_position_embeddings",
		"model_max_length",
		"max_sequence_length",
		"max_seq_len",
		"seq_length",
		"n_positions",
		"n_ctx",
	} {
		if value := int64Value(summaryValue(config, key)); value > 0 {
			return value
		}
	}

	return 0
}

func detectQuantization(config map[string]any) string {
	quantizationConfig, _ := config["quantization_config"].(map[string]any)
	if quantizationConfig == nil {
		return ""
	}

	method := strings.ToLower(stringValue(quantizationConfig["quant_method"]))
	bits := int64Value(quantizationConfig["bits"])
	loadIn4Bit, _ := quantizationConfig["load_in_4bit"].(bool)
	loadIn8Bit, _ := quantizationConfig["load_in_8bit"].(bool)
	bnb4BitType := strings.ToLower(stringValue(quantizationConfig["bnb_4bit_quant_type"]))

	switch {
	case method != "" && bits > 0:
		return fmt.Sprintf("%s-%dbit", method, bits)
	case method != "" && loadIn4Bit:
		return method + "-4bit"
	case method != "" && loadIn8Bit:
		return method + "-8bit"
	case bits > 0:
		return fmt.Sprintf("%dbit", bits)
	case bnb4BitType != "" && loadIn4Bit:
		return "bnb-" + bnb4BitType
	case loadIn4Bit:
		return "4bit"
	case loadIn8Bit:
		return "8bit"
	case method != "":
		return method
	default:
		return ""
	}
}

func detectPrecision(config map[string]any, quantization string) string {
	bytesPerParameter := profilecommon.BytesPerParameter("", quantization)
	switch bytesPerParameter {
	case 0.5:
		return "int4"
	case 1:
		return "int8"
	}

	switch strings.ToLower(stringValue(summaryValue(config, "torch_dtype"))) {
	case "bfloat16", "bf16":
		return "bf16"
	case "float16", "fp16", "half", "f16":
		return "fp16"
	case "float32", "fp32":
		return "fp32"
	default:
		return ""
	}
}

func estimateParameterCount(config map[string]any) int64 {
	for _, key := range []string{"num_parameters", "parameter_count"} {
		if direct := int64Value(config[key]); direct > 0 {
			return direct
		}
	}

	hiddenSize := int64Value(summaryValue(config, "hidden_size"))
	intermediateSize := int64Value(summaryValue(config, "intermediate_size"))
	numLayers := int64Value(summaryValue(config, "num_hidden_layers"))
	numAttentionHeads := int64Value(summaryValue(config, "num_attention_heads"))
	numKeyValueHeads := int64Value(summaryValue(config, "num_key_value_heads"))
	vocabSize := int64Value(summaryValue(config, "vocab_size"))
	if hiddenSize <= 0 || intermediateSize <= 0 || numLayers <= 0 || vocabSize <= 0 {
		return 0
	}

	kvHiddenSize := hiddenSize
	if numAttentionHeads > 0 && numKeyValueHeads > 0 {
		headSize := hiddenSize / numAttentionHeads
		kvHiddenSize = numKeyValueHeads * headSize
	}

	embedding := vocabSize * hiddenSize
	attention := (2 * hiddenSize * hiddenSize) + (2 * hiddenSize * kvHiddenSize)
	mlp := 3 * hiddenSize * intermediateSize
	perLayer := attention + mlp

	return embedding + (numLayers * perLayer)
}

func compatibleRuntimes(runtimeEngines []string) []string {
	if len(runtimeEngines) > 0 {
		return profilecommon.UniqueStrings(runtimeEngines)
	}
	return []string{"KServe", "KubeRay"}
}

func compatiblePrecisions(precision string) []string {
	if strings.TrimSpace(precision) == "" {
		return nil
	}
	return []string{strings.TrimSpace(precision)}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
