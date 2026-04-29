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
	normalized := normalize(strings.TrimSpace(task))
	for _, rule := range capabilityRules {
		if slices.Contains(rule.tasks, normalized) {
			return capabilityWithFeatures(rule.endpoints, rule.features...)
		}
	}
	return Capabilities{}
}

func DeclaredSourceTasks(tasks ...string) []string {
	result := make([]string, 0, len(tasks))
	for _, task := range tasks {
		normalized := normalize(task)
		if normalized == "" || slices.Contains(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

type capabilityRule struct {
	tasks     []string
	endpoints []modelsv1alpha1.ModelEndpointType
	features  []modelsv1alpha1.ModelFeatureType
}

var capabilityRules = []capabilityRule{
	{
		tasks: []string{"text-generation", "text2text-generation", "summarization", "conversational"},
		endpoints: []modelsv1alpha1.ModelEndpointType{
			modelsv1alpha1.ModelEndpointTypeChat,
			modelsv1alpha1.ModelEndpointTypeTextGeneration,
		},
	},
	{
		tasks:     []string{"feature-extraction", "embeddings", "text-embeddings-inference", "sentence-similarity"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeEmbeddings},
	},
	{
		tasks:     []string{"rerank", "reranker", "text-ranking"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeRerank},
	},
	{
		tasks:     []string{"automatic-speech-recognition", "speech-to-text"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeSpeechToText},
		features:  []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeAudioInput},
	},
	{
		tasks:     []string{"text-to-speech"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeTextToSpeech},
		features:  []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeAudioOutput},
	},
	{
		tasks:     []string{"text-to-audio", "text-to-music", "audio-generation"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeAudioGeneration},
		features:  []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeAudioOutput},
	},
	{
		tasks:     []string{"translation", "translation_xx_to_yy"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeTranslation},
	},
	{
		tasks:     []string{"image-classification", "zero-shot-image-classification"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeImageClassification},
		features:  []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeVisionInput},
	},
	{
		tasks:     []string{"object-detection", "zero-shot-object-detection"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeObjectDetection},
		features:  []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeVisionInput},
	},
	{
		tasks:     []string{"image-segmentation"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeImageSegmentation},
		features:  []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeVisionInput},
	},
	{
		tasks: []string{"image-to-text", "image-text-to-text"},
		endpoints: []modelsv1alpha1.ModelEndpointType{
			modelsv1alpha1.ModelEndpointTypeChat,
			modelsv1alpha1.ModelEndpointTypeImageToText,
		},
		features: []modelsv1alpha1.ModelFeatureType{
			modelsv1alpha1.ModelFeatureTypeVisionInput,
			modelsv1alpha1.ModelFeatureTypeMultiModalInput,
		},
	},
	{
		tasks:     []string{"visual-question-answering", "document-question-answering"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeVisualQuestionAnswering},
		features: []modelsv1alpha1.ModelFeatureType{
			modelsv1alpha1.ModelFeatureTypeVisionInput,
			modelsv1alpha1.ModelFeatureTypeMultiModalInput,
		},
	},
	{
		tasks:     []string{"text-to-image", "image-generation", "unconditional-image-generation"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeImageGeneration},
		features:  []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeImageOutput},
	},
	{
		tasks:     []string{"image-to-image", "image-variation", "inpainting", "image-inpainting"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeImageGeneration},
		features: []modelsv1alpha1.ModelFeatureType{
			modelsv1alpha1.ModelFeatureTypeVisionInput,
			modelsv1alpha1.ModelFeatureTypeImageOutput,
		},
	},
	{
		tasks:     []string{"text-to-video", "video-generation"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeVideoGeneration},
		features:  []modelsv1alpha1.ModelFeatureType{modelsv1alpha1.ModelFeatureTypeVideoOutput},
	},
	{
		tasks:     []string{"image-to-video"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeVideoGeneration},
		features: []modelsv1alpha1.ModelFeatureType{
			modelsv1alpha1.ModelFeatureTypeVisionInput,
			modelsv1alpha1.ModelFeatureTypeVideoOutput,
		},
	},
	{
		tasks:     []string{"video-to-video"},
		endpoints: []modelsv1alpha1.ModelEndpointType{modelsv1alpha1.ModelEndpointTypeVideoGeneration},
		features: []modelsv1alpha1.ModelFeatureType{
			modelsv1alpha1.ModelFeatureTypeVideoInput,
			modelsv1alpha1.ModelFeatureTypeVideoOutput,
		},
	},
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
