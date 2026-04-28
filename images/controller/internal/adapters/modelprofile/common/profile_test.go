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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestEndpointTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		task      string
		endpoints []string
		features  []string
	}{
		{
			name:      "chat generation",
			task:      "text-generation",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeChat), string(modelsv1alpha1.ModelEndpointTypeTextGeneration)},
		},
		{
			name:      "embeddings",
			task:      "embeddings",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeEmbeddings)},
		},
		{
			name:      "rerank",
			task:      "text-ranking",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeRerank)},
		},
		{
			name:      "speech to text",
			task:      "automatic-speech-recognition",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeSpeechToText)},
			features:  []string{string(modelsv1alpha1.ModelFeatureTypeAudioInput)},
		},
		{
			name:      "text to speech",
			task:      "text-to-speech",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeTextToSpeech)},
			features:  []string{string(modelsv1alpha1.ModelFeatureTypeAudioOutput)},
		},
		{
			name:      "cv",
			task:      "object-detection",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeObjectDetection)},
			features:  []string{string(modelsv1alpha1.ModelFeatureTypeVisionInput)},
		},
		{
			name:      "image generation",
			task:      "text-to-image",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeImageGeneration)},
			features:  []string{string(modelsv1alpha1.ModelFeatureTypeImageOutput)},
		},
		{
			name:      "multimodal",
			task:      "image-text-to-text",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeChat), string(modelsv1alpha1.ModelEndpointTypeImageToText)},
			features:  []string{string(modelsv1alpha1.ModelFeatureTypeVisionInput), string(modelsv1alpha1.ModelFeatureTypeMultiModalInput)},
		},
		{
			name:      "translation",
			task:      "translation",
			endpoints: []string{string(modelsv1alpha1.ModelEndpointTypeTranslation)},
		},
		{name: "unknown", task: "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			capabilities := ResolveCapabilities(tc.task)
			if !stringSlicesEqual(capabilities.EndpointTypes, tc.endpoints) {
				t.Fatalf("unexpected endpoint types %#v", capabilities.EndpointTypes)
			}
			if !stringSlicesEqual(capabilities.Features, tc.features) {
				t.Fatalf("unexpected features %#v", capabilities.Features)
			}
		})
	}
}

func stringSlicesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestEstimatedWorkingSetGiB(t *testing.T) {
	t.Parallel()

	workingSet := EstimatedWorkingSetGiB(32<<30, 0, "", "")
	if got, want := workingSet, int64(40); got != want {
		t.Fatalf("unexpected working set %d", got)
	}
}

func TestEstimateParameterCountFromBytes(t *testing.T) {
	t.Parallel()

	if got, want := EstimateParameterCountFromBytes(8<<30, "", "q4_k_m"), int64(17179869184); got != want {
		t.Fatalf("unexpected parameter count %d", got)
	}
}
