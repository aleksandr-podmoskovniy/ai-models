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

import "strings"

func resolveTask(config map[string]any, architecture, explicitTask, sourceHint string) string {
	if task := strings.TrimSpace(explicitTask); task != "" {
		return task
	}
	if inferred := inferTaskFromCheckpoint(config, architecture); inferred != "" {
		return inferred
	}
	return strings.TrimSpace(sourceHint)
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
