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
	"strings"

	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func resolveTask(
	config map[string]any,
	architecture string,
	explicitTask string,
	sourceDeclaredTask string,
	sourceHint string,
) (string, publicationdata.ProfileConfidence) {
	if task := strings.TrimSpace(explicitTask); task != "" {
		return task, publicationdata.ProfileConfidenceExact
	}
	if task := stableDeclaredTask(sourceDeclaredTask); task != "" {
		return task, publicationdata.ProfileConfidenceDeclared
	}
	if broadTask(sourceDeclaredTask) == "any-to-any" || broadTask(sourceHint) == "any-to-any" {
		if inferred := inferMultimodalTaskFromCheckpoint(config, architecture); inferred != "" {
			return inferred, publicationdata.ProfileConfidenceDerived
		}
	}
	if inferred := inferTaskFromCheckpoint(config, architecture); inferred != "" {
		return inferred, publicationdata.ProfileConfidenceDerived
	}
	if hint := strings.TrimSpace(sourceHint); hint != "" {
		return hint, publicationdata.ProfileConfidenceHint
	}
	return "", ""
}

func stableDeclaredTask(task string) string {
	normalized := strings.ToLower(strings.TrimSpace(task))
	if normalized == "any-to-any" {
		return ""
	}
	return strings.TrimSpace(task)
}

func broadTask(task string) string {
	return strings.ToLower(strings.TrimSpace(task))
}

func inferMultimodalTaskFromCheckpoint(config map[string]any, architecture string) string {
	if hasVisionConfig(config) || strings.Contains(strings.ToLower(architecture), "vision") {
		return "image-text-to-text"
	}
	return ""
}

func hasVisionConfig(config map[string]any) bool {
	for _, key := range []string{"vision_config", "visual_config", "vision_tower", "image_token_index"} {
		if _, found := config[key]; found {
			return true
		}
	}
	return false
}

func inferTaskFromCheckpoint(config map[string]any, architecture string) string {
	for _, candidate := range []string{architecture, firstNonEmpty(stringSlice(config["architectures"])...)} {
		switch normalizeArchitecture(candidate) {
		case "Qwen3", "Qwen2", "Qwen2Moe", "Llama", "Mistral", "Mixtral", "Phi3", "Phi", "Gemma", "GPTNeoX", "Falcon":
			return "text-generation"
		}
	}

	normalizedArchitecture := strings.TrimSpace(architecture)
	switch {
	case strings.HasSuffix(normalizedArchitecture, "ForCausalLM"):
		return "text-generation"
	case strings.HasSuffix(normalizedArchitecture, "ForConditionalGeneration"):
		return "text2text-generation"
	case strings.HasSuffix(normalizedArchitecture, "ForSequenceClassification"):
		return "text-classification"
	case strings.HasSuffix(normalizedArchitecture, "ForTokenClassification"):
		return "token-classification"
	case strings.HasSuffix(normalizedArchitecture, "ForQuestionAnswering"):
		return "question-answering"
	case strings.HasSuffix(normalizedArchitecture, "ForMaskedLM"):
		return "fill-mask"
	default:
		return ""
	}
}
