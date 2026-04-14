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

package publishstate

import modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"

func inferModelType(task string) modelsv1alpha1.ModelType {
	switch normalizeValue(task) {
	case "text-generation", "text2text-generation", "summarization", "conversational":
		return modelsv1alpha1.ModelTypeLLM
	case "feature-extraction", "embeddings", "text-embeddings-inference", "sentence-similarity":
		return modelsv1alpha1.ModelTypeEmbeddings
	case "rerank", "reranker", "text-ranking":
		return modelsv1alpha1.ModelTypeReranker
	case "automatic-speech-recognition", "speech-to-text":
		return modelsv1alpha1.ModelTypeSpeechToText
	case "translation":
		return modelsv1alpha1.ModelTypeTranslation
	default:
		return ""
	}
}

func inferEndpointTypes(task string) []string {
	switch inferModelType(task) {
	case modelsv1alpha1.ModelTypeLLM:
		return []string{
			string(modelsv1alpha1.ModelEndpointTypeChat),
			string(modelsv1alpha1.ModelEndpointTypeTextGeneration),
		}
	case modelsv1alpha1.ModelTypeEmbeddings:
		return []string{string(modelsv1alpha1.ModelEndpointTypeEmbeddings)}
	case modelsv1alpha1.ModelTypeReranker:
		return []string{string(modelsv1alpha1.ModelEndpointTypeRerank)}
	case modelsv1alpha1.ModelTypeSpeechToText:
		return []string{string(modelsv1alpha1.ModelEndpointTypeSpeechToText)}
	case modelsv1alpha1.ModelTypeTranslation:
		return []string{string(modelsv1alpha1.ModelEndpointTypeTranslation)}
	default:
		return nil
	}
}

func draftModelRefs(policy *modelsv1alpha1.ModelOptimizationPolicy) []modelsv1alpha1.ModelReference {
	if policy == nil || policy.SpeculativeDecoding == nil {
		return nil
	}
	return policy.SpeculativeDecoding.DraftModelRefs
}
