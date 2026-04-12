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
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProfile(t *testing.T) {
	t.Parallel()

	checkpointDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(checkpointDir, "model-00001-of-00002.safetensors"), []byte("weights-1"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkpointDir, "model-00002-of-00002.safetensors"), []byte("weights-2"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkpointDir, "config.json"), []byte(`{
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
}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolved, err := Resolve(Input{
		CheckpointDir:  checkpointDir,
		Task:           "text-generation",
		RuntimeEngines: []string{"KServe"},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Family, "qwen3"; got != want {
		t.Fatalf("unexpected family %q", got)
	}
	if got := resolved.ParameterCount; got <= 0 {
		t.Fatalf("expected parameter count, got %d", got)
	}
	if got, want := resolved.ContextWindowTokens, int64(32768); got != want {
		t.Fatalf("unexpected context window %d", got)
	}
	if len(resolved.CompatibleRuntimes) != 1 || resolved.CompatibleRuntimes[0] != "KServe" {
		t.Fatalf("unexpected compatible runtimes %#v", resolved.CompatibleRuntimes)
	}
	if len(resolved.SupportedEndpointTypes) == 0 {
		t.Fatal("expected supported endpoint types")
	}
	if len(resolved.CompatibleAcceleratorVendors) != 2 {
		t.Fatalf("unexpected compatible accelerator vendors %#v", resolved.CompatibleAcceleratorVendors)
	}
	if len(resolved.CompatiblePrecisions) != 1 || resolved.CompatiblePrecisions[0] != "bf16" {
		t.Fatalf("unexpected compatible precisions %#v", resolved.CompatiblePrecisions)
	}
	if got, want := resolved.MinimumLaunch.PlacementType, "GPU"; got != want {
		t.Fatalf("unexpected placement type %q", got)
	}
	if resolved.MinimumLaunch.AcceleratorMemoryGiB <= 0 {
		t.Fatalf("unexpected minimum launch %#v", resolved.MinimumLaunch)
	}
}

func TestResolveProfileInfersTaskFromArchitecture(t *testing.T) {
	t.Parallel()

	checkpointDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(checkpointDir, "model.safetensors"), []byte("weights"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkpointDir, "config.json"), []byte(`{
  "architectures":["Qwen3ForCausalLM"],
  "torch_dtype":"bfloat16",
  "text_config":{"max_position_embeddings":32768}
}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolved, err := Resolve(Input{
		CheckpointDir: checkpointDir,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Task, "text-generation"; got != want {
		t.Fatalf("unexpected task %q", got)
	}
	if len(resolved.SupportedEndpointTypes) == 0 {
		t.Fatal("expected endpoint types from inferred task")
	}
}

func TestResolveProfileUsesTaskHintAsFallback(t *testing.T) {
	t.Parallel()

	checkpointDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(checkpointDir, "model.safetensors"), []byte("weights"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkpointDir, "config.json"), []byte(`{
  "architectures":["CustomModel"],
  "torch_dtype":"bfloat16"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolved, err := Resolve(Input{
		CheckpointDir: checkpointDir,
		TaskHint:      "feature-extraction",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Task, "feature-extraction"; got != want {
		t.Fatalf("unexpected task %q", got)
	}
	if len(resolved.SupportedEndpointTypes) == 0 || resolved.SupportedEndpointTypes[0] != "OpenAIEmbeddings" {
		t.Fatalf("unexpected endpoint types %#v", resolved.SupportedEndpointTypes)
	}
}

func TestResolveProfileDoesNotInferFamilyFromSourceRepoID(t *testing.T) {
	t.Parallel()

	checkpointDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(checkpointDir, "model.safetensors"), []byte("weights"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkpointDir, "config.json"), []byte(`{
  "torch_dtype":"bfloat16"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolved, err := Resolve(Input{
		CheckpointDir: checkpointDir,
		TaskHint:      "text-generation",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Family, ""; got != want {
		t.Fatalf("unexpected family %q", got)
	}
}
