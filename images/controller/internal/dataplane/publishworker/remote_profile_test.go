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

package publishworker

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
)

func TestResolveRemoteProfileSafetensorsSummary(t *testing.T) {
	t.Parallel()

	resolved, err := resolveRemoteProfile(Options{Task: "text-generation"}, sourcefetch.RemoteResult{
		InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
		Fallbacks: sourcefetch.RemoteProfileFallbacks{
			TaskHint: "feature-extraction",
		},
		ProfileSummary: &sourcefetch.RemoteProfileSummary{
			ConfigPayload: []byte(`{
  "model_type":"qwen3",
  "architectures":["Qwen3ForCausalLM"],
  "torch_dtype":"bfloat16",
  "text_config":{
    "hidden_size":4096,
    "intermediate_size":11008,
    "num_hidden_layers":32,
    "num_attention_heads":32,
    "num_key_value_heads":8,
    "max_position_embeddings":32768,
    "vocab_size":151936
  }
}`),
			WeightBytes:            24,
			LargestWeightFileBytes: 14,
			WeightFileCount:        2,
		},
	})
	if err != nil {
		t.Fatalf("resolveRemoteProfile() error = %v", err)
	}
	if resolved == nil {
		t.Fatal("expected resolved remote profile")
	}
	if got, want := resolved.Format, "Safetensors"; got != want {
		t.Fatalf("unexpected format %q", got)
	}
	if got, want := resolved.Task, "text-generation"; got != want {
		t.Fatalf("unexpected task %q", got)
	}
	if got, want := resolved.Footprint.LargestWeightFileBytes, int64(14); got != want {
		t.Fatalf("unexpected largest weight bytes %d", got)
	}
	if got, want := resolved.Footprint.ShardCount, int64(2); got != want {
		t.Fatalf("unexpected shard count %d", got)
	}
}

func TestResolveRemoteProfileGGUFSummary(t *testing.T) {
	t.Parallel()

	resolved, err := resolveRemoteProfile(Options{Task: "text-generation"}, sourcefetch.RemoteResult{
		InputFormat: modelsv1alpha1.ModelInputFormatGGUF,
		ProfileSummary: &sourcefetch.RemoteProfileSummary{
			ModelFileName:  "deepseek-r1-8b-q4_k_m.gguf",
			ModelSizeBytes: 42,
		},
	})
	if err != nil {
		t.Fatalf("resolveRemoteProfile() error = %v", err)
	}
	if resolved == nil {
		t.Fatal("expected resolved remote profile")
	}
	if got, want := resolved.Format, "GGUF"; got != want {
		t.Fatalf("unexpected format %q", got)
	}
	if got, want := resolved.Task, "text-generation"; got != want {
		t.Fatalf("unexpected task %q", got)
	}
}
