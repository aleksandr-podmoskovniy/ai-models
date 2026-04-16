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
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func EndpointTypes(task string) []string {
	switch normalize(strings.TrimSpace(task)) {
	case "text-generation", "text2text-generation", "summarization", "conversational":
		return []string{
			string(modelsv1alpha1.ModelEndpointTypeChat),
			string(modelsv1alpha1.ModelEndpointTypeTextGeneration),
		}
	case "feature-extraction", "embeddings", "text-embeddings-inference", "sentence-similarity":
		return []string{string(modelsv1alpha1.ModelEndpointTypeEmbeddings)}
	case "rerank", "reranker", "text-ranking":
		return []string{string(modelsv1alpha1.ModelEndpointTypeRerank)}
	case "automatic-speech-recognition", "speech-to-text":
		return []string{string(modelsv1alpha1.ModelEndpointTypeSpeechToText)}
	case "translation":
		return []string{string(modelsv1alpha1.ModelEndpointTypeTranslation)}
	default:
		return nil
	}
}

func GPUVendors() []string {
	return []string{"NVIDIA", "AMD"}
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

func GPUWorkingSetGiB(modelBytes, parameterCount int64, precision, quantization string) int64 {
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

func MinimumGPULaunch(workingSetGiB int64) publicationdata.MinimumLaunch {
	if workingSetGiB <= 0 {
		return publicationdata.MinimumLaunch{}
	}

	acceleratorCount := int64(math.Ceil(float64(workingSetGiB) / 80.0))
	if acceleratorCount <= 0 {
		acceleratorCount = 1
	}
	perAcceleratorGiB := int64(math.Ceil(float64(workingSetGiB) / float64(acceleratorCount)))
	if perAcceleratorGiB <= 0 {
		perAcceleratorGiB = 1
	}

	return publicationdata.MinimumLaunch{
		PlacementType:        "GPU",
		AcceleratorCount:     acceleratorCount,
		AcceleratorMemoryGiB: perAcceleratorGiB,
		SharingMode:          "Exclusive",
	}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
