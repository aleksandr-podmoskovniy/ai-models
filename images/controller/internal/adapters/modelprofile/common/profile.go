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

package common

import (
	"math"
	"slices"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

type Capabilities struct {
	EndpointTypes []string
	Features      []string
}

func ResolveCapabilities(task string) Capabilities {
	switch normalize(strings.TrimSpace(task)) {
	case "text-generation", "text2text-generation", "summarization", "conversational":
		return capability(
			modelsv1alpha1.ModelEndpointTypeChat,
			modelsv1alpha1.ModelEndpointTypeTextGeneration,
		)
	case "feature-extraction", "embeddings", "text-embeddings-inference", "sentence-similarity":
		return capability(modelsv1alpha1.ModelEndpointTypeEmbeddings)
	case "rerank", "reranker", "text-ranking":
		return capability(modelsv1alpha1.ModelEndpointTypeRerank)
	case "automatic-speech-recognition", "speech-to-text":
		return capabilityWithFeatures(
			[]modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeSpeechToText},
			modelsv1alpha1.ModelFeatureTypeAudioInput,
		)
	case "text-to-speech", "text-to-audio":
		return capabilityWithFeatures(
			[]modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeTextToSpeech},
			modelsv1alpha1.ModelFeatureTypeAudioOutput,
		)
	case "translation", "translation_xx_to_yy":
		return capability(modelsv1alpha1.ModelEndpointTypeTranslation)
	case "image-classification", "zero-shot-image-classification":
		return capabilityWithFeatures(
			[]modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeImageClassification},
			modelsv1alpha1.ModelFeatureTypeVisionInput,
		)
	case "object-detection", "zero-shot-object-detection":
		return capabilityWithFeatures(
			[]modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeObjectDetection},
			modelsv1alpha1.ModelFeatureTypeVisionInput,
		)
	case "image-segmentation":
		return capabilityWithFeatures(
			[]modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeImageSegmentation},
			modelsv1alpha1.ModelFeatureTypeVisionInput,
		)
	case "image-to-text", "image-text-to-text":
		return capabilityWithFeatures(
			[]modelsv1alpha1.ModelEndpointType{
				modelsv1alpha1.ModelEndpointTypeChat,
				modelsv1alpha1.ModelEndpointTypeImageToText,
			},
			modelsv1alpha1.ModelFeatureTypeVisionInput,
			modelsv1alpha1.ModelFeatureTypeMultiModalInput,
		)
	case "visual-question-answering", "document-question-answering":
		return capabilityWithFeatures(
			[]modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeVisualQuestionAnswering},
			modelsv1alpha1.ModelFeatureTypeVisionInput,
			modelsv1alpha1.ModelFeatureTypeMultiModalInput,
		)
	case "text-to-image", "image-generation":
		return capabilityWithFeatures(
			[]modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeImageGeneration},
			modelsv1alpha1.ModelFeatureTypeImageOutput,
		)
	default:
		return Capabilities{}
	}
}

func capability(endpoints ...modelsv1alpha1.ModelEndpointType) Capabilities {
	return capabilityWithFeatures(endpoints)
}

func capabilityWithFeatures(endpoints []modelsv1alpha1.ModelEndpointType, features ...modelsv1alpha1.ModelFeatureType) Capabilities {
	resolved := Capabilities{
		EndpointTypes: make([]string, 0, len(endpoints)),
		Features:      make([]string, 0, len(features)),
	}
	for _, endpoint := range endpoints {
		value := string(endpoint)
		if value != "" && !slices.Contains(resolved.EndpointTypes, value) {
			resolved.EndpointTypes = append(resolved.EndpointTypes, value)
		}
	}
	for _, feature := range features {
		value := string(feature)
		if value != "" && !slices.Contains(resolved.Features, value) {
			resolved.Features = append(resolved.Features, value)
		}
	}
	return resolved
}

func EstimateParameterCountFromBytes(modelBytes int64, precision, quantization string) int64 {
	if modelBytes <= 0 {
		return 0
	}

	bytesPerParameter := BytesPerParameter(precision, quantization)
	if bytesPerParameter <= 0 {
		return 0
	}

	return int64(float64(modelBytes) / bytesPerParameter)
}

func BytesPerParameter(precision, quantization string) float64 {
	normalizedQuantization := normalize(quantization)
	switch {
	case strings.Contains(normalizedQuantization, "nf4"),
		strings.Contains(normalizedQuantization, "fp4"),
		strings.Contains(normalizedQuantization, "4bit"),
		strings.Contains(normalizedQuantization, "int4"),
		strings.HasPrefix(normalizedQuantization, "q4"),
		strings.HasPrefix(normalizedQuantization, "iq4"):
		return 0.5
	case strings.Contains(normalizedQuantization, "8bit"),
		strings.Contains(normalizedQuantization, "int8"),
		strings.HasPrefix(normalizedQuantization, "q8"):
		return 1
	}

	switch normalize(precision) {
	case "int4":
		return 0.5
	case "int8":
		return 1
	case "bf16", "fp16", "f16":
		return 2
	case "fp32":
		return 4
	default:
		return 2
	}
}

func EstimatedWorkingSetGiB(modelBytes, parameterCount int64, precision, quantization string) int64 {
	estimatedBytes := modelBytes
	if estimatedBytes <= 0 && parameterCount > 0 {
		estimatedBytes = int64(float64(parameterCount) * BytesPerParameter(precision, quantization))
	}
	if estimatedBytes <= 0 {
		return 0
	}

	const overheadFactor = 1.25
	workingSetGiB := int64(math.Ceil((float64(estimatedBytes) * overheadFactor) / float64(1<<30)))
	if workingSetGiB <= 0 {
		return 1
	}
	return workingSetGiB
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
