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
	"fmt"
	"strings"

	profilecommon "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/common"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func resolveArchitecture(config map[string]any) string {
	architectures := stringSlice(config["architectures"])
	return firstNonEmpty(architectures...)
}

func resolveFamily(config map[string]any, architecture string) (string, publicationdata.ProfileConfidence) {
	if family := strings.TrimSpace(stringValue(config["model_type"])); family != "" {
		return strings.ToLower(family), publicationdata.ProfileConfidenceExact
	}

	if normalized := normalizeArchitecture(architecture); normalized != "" {
		return strings.ToLower(normalized), publicationdata.ProfileConfidenceDerived
	}

	return "", ""
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

func resolveParameterCount(
	config map[string]any,
	weightBytes int64,
	precision string,
	quantization string,
) (int64, publicationdata.ProfileConfidence) {
	for _, key := range []string{"num_parameters", "parameter_count"} {
		if direct := int64Value(config[key]); direct > 0 {
			return direct, publicationdata.ProfileConfidenceExact
		}
	}

	hiddenSize := int64Value(summaryValue(config, "hidden_size"))
	intermediateSize := int64Value(summaryValue(config, "intermediate_size"))
	numLayers := int64Value(summaryValue(config, "num_hidden_layers"))
	numAttentionHeads := int64Value(summaryValue(config, "num_attention_heads"))
	numKeyValueHeads := int64Value(summaryValue(config, "num_key_value_heads"))
	vocabSize := int64Value(summaryValue(config, "vocab_size"))
	if hiddenSize <= 0 || intermediateSize <= 0 || numLayers <= 0 || vocabSize <= 0 {
		return estimatedParameterCount(weightBytes, precision, quantization)
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

	return embedding + (numLayers * perLayer), publicationdata.ProfileConfidenceEstimated
}

func estimatedParameterCount(
	weightBytes int64,
	precision string,
	quantization string,
) (int64, publicationdata.ProfileConfidence) {
	parameterCount := profilecommon.EstimateParameterCountFromBytes(weightBytes, precision, quantization)
	if parameterCount <= 0 {
		return 0, ""
	}
	return parameterCount, publicationdata.ProfileConfidenceEstimated
}
