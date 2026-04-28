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
	"testing"

	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestResolveProfileDemotesBroadAnyToAnyWithoutCheckpointEvidence(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveSummary(SummaryInput{
		ConfigPayload: []byte(`{
  "model_type":"gemma",
  "architectures":["GemmaForConditionalGeneration"],
  "torch_dtype":"bfloat16"
}`),
		WeightBytes:        24,
		WeightFileCount:    1,
		SourceDeclaredTask: "any-to-any",
	})
	if err != nil {
		t.Fatalf("ResolveSummary() error = %v", err)
	}
	if got, want := resolved.Task, "text-generation"; got != want {
		t.Fatalf("unexpected task %q", got)
	}
	if got, want := resolved.TaskConfidence, "Derived"; string(got) != want {
		t.Fatalf("unexpected task confidence %q", got)
	}
	if got, want := resolved.SupportedEndpointTypes, []string{"Chat", "TextGeneration"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected endpoint types %#v", got)
	}
	if len(resolved.SupportedFeatures) != 0 {
		t.Fatalf("unexpected features %#v", resolved.SupportedFeatures)
	}
}

func TestResolveProfileProjectsBroadAnyToAnyWithVisionCheckpointEvidence(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveSummary(SummaryInput{
		ConfigPayload: []byte(`{
  "model_type":"gemma",
  "architectures":["GemmaForConditionalGeneration"],
  "torch_dtype":"bfloat16",
  "vision_config":{"image_size":896}
}`),
		WeightBytes:        24,
		WeightFileCount:    1,
		SourceDeclaredTask: "any-to-any",
	})
	if err != nil {
		t.Fatalf("ResolveSummary() error = %v", err)
	}
	assertVisionAnyToAnyProfile(t, resolved)
}

func TestResolveProfileProjectsAnyToAnyHintWithVisionCheckpointEvidence(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveSummary(SummaryInput{
		ConfigPayload: []byte(`{
  "model_type":"gemma",
  "architectures":["GemmaForConditionalGeneration"],
  "torch_dtype":"bfloat16",
  "vision_config":{"image_size":896}
}`),
		WeightBytes:     24,
		WeightFileCount: 1,
		TaskHint:        "any-to-any",
	})
	if err != nil {
		t.Fatalf("ResolveSummary() error = %v", err)
	}
	assertVisionAnyToAnyProfile(t, resolved)
}

func TestResolveProfileDetectsToolCallingTemplate(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveSummary(SummaryInput{
		ConfigPayload: []byte(`{
  "model_type":"qwen2",
  "architectures":["Qwen2ForCausalLM"],
  "torch_dtype":"bfloat16"
}`),
		TokenizerConfigPayload: []byte(`{
  "chat_template":"{%- if tools %}{%- for tool in tools %}{{ tool | tojson }}{%- endfor %}{%- endif %}"
}`),
		WeightBytes:     24,
		WeightFileCount: 1,
	})
	if err != nil {
		t.Fatalf("ResolveSummary() error = %v", err)
	}
	if got, want := resolved.SupportedFeatures, []string{"ToolCalling"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected features %#v", got)
	}
}

func assertVisionAnyToAnyProfile(t *testing.T, resolved publicationdata.ResolvedProfile) {
	t.Helper()

	if got, want := resolved.Task, "image-text-to-text"; got != want {
		t.Fatalf("unexpected task %q", got)
	}
	if got, want := resolved.TaskConfidence, "Derived"; string(got) != want {
		t.Fatalf("unexpected task confidence %q", got)
	}
	if got, want := resolved.SupportedEndpointTypes, []string{"Chat", "ImageToText"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected endpoint types %#v", got)
	}
	if got, want := resolved.SupportedFeatures, []string{"VisionInput", "MultiModalInput"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected features %#v", got)
	}
}
