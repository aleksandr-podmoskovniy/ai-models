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

package diffusers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSummaryProjectsTextToVideo(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveSummary(SummaryInput{
		ModelIndexPayload:      []byte(`{"_class_name":"TextToVideoSDPipeline"}`),
		WeightBytes:            128,
		LargestWeightFileBytes: 64,
		WeightFileCount:        2,
	})
	if err != nil {
		t.Fatalf("ResolveSummary() error = %v", err)
	}
	if got, want := resolved.Format, "Diffusers"; got != want {
		t.Fatalf("unexpected format %q", got)
	}
	if got, want := resolved.Task, "text-to-video"; got != want {
		t.Fatalf("unexpected task %q", got)
	}
	if got, want := resolved.TaskConfidence, "Derived"; string(got) != want {
		t.Fatalf("unexpected task confidence %q", got)
	}
	if got, want := resolved.SupportedEndpointTypes, []string{"VideoGeneration"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected endpoints %#v", got)
	}
	if got, want := resolved.SupportedFeatures, []string{"VideoOutput"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected features %#v", got)
	}
}

func TestResolveSummaryProjectsImageToVideoFromDeclaredTask(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveSummary(SummaryInput{
		ModelIndexPayload:  []byte(`{"_class_name":"CogVideoXPipeline"}`),
		WeightBytes:        128,
		WeightFileCount:    1,
		SourceDeclaredTask: "image-to-video",
	})
	if err != nil {
		t.Fatalf("ResolveSummary() error = %v", err)
	}
	if got, want := resolved.TaskConfidence, "Declared"; string(got) != want {
		t.Fatalf("unexpected task confidence %q", got)
	}
	if got, want := resolved.SupportedEndpointTypes, []string{"VideoGeneration"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected endpoints %#v", got)
	}
	if got, want := resolved.SupportedFeatures, []string{"VisionInput", "VideoOutput"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected features %#v", got)
	}
}

func TestResolveSummaryDoesNotProjectUnknownPipelineWithoutReliableTask(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveSummary(SummaryInput{
		ModelIndexPayload: []byte(`{"_class_name":"CustomDiffusionPipeline"}`),
		WeightBytes:       128,
		WeightFileCount:   1,
		TaskHint:          "text-to-video",
	})
	if err != nil {
		t.Fatalf("ResolveSummary() error = %v", err)
	}
	if got, want := resolved.TaskConfidence, "Hint"; string(got) != want {
		t.Fatalf("unexpected task confidence %q", got)
	}
	if len(resolved.SupportedEndpointTypes) != 0 {
		t.Fatalf("hint-only task must not project endpoints: %#v", resolved.SupportedEndpointTypes)
	}
}

func TestResolveCountsDiffusersBinWeights(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeProfileTestFile(t, filepath.Join(root, "model_index.json"), `{"_class_name":"TextToVideoSDPipeline"}`)
	writeProfileTestFile(t, filepath.Join(root, "transformer", "diffusion_pytorch_model.bin"), "video-weights")
	writeProfileTestFile(t, filepath.Join(root, "text_encoder", "pytorch_model.bin"), "text-weights")

	resolved, err := Resolve(Input{ModelDir: root})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Footprint.WeightsBytes, int64(len("video-weights")+len("text-weights")); got != want {
		t.Fatalf("unexpected weight bytes %d", got)
	}
	if got, want := resolved.Footprint.ShardCount, int64(2); got != want {
		t.Fatalf("unexpected shard count %d", got)
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

func writeProfileTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
