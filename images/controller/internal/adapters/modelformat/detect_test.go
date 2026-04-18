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

package modelformat

import (
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestDetectDirFormatSafetensors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.json"), `{"model_type":"qwen3"}`)
	writeTestFile(t, filepath.Join(root, "model.safetensors"), "weights")
	writeTestFile(t, filepath.Join(root, "README.md"), "# docs")

	format, err := DetectDirFormat(root)
	if err != nil {
		t.Fatalf("DetectDirFormat() error = %v", err)
	}
	if got, want := format, modelsv1alpha1.ModelInputFormatSafetensors; got != want {
		t.Fatalf("unexpected format %q", got)
	}
}

func TestDetectDirFormatDropsHiddenDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".cache", "ignored.gguf"), "weights")
	writeTestFile(t, filepath.Join(root, "model.gguf"), "weights")

	format, err := DetectDirFormat(root)
	if err != nil {
		t.Fatalf("DetectDirFormat() error = %v", err)
	}
	if got, want := format, modelsv1alpha1.ModelInputFormatGGUF; got != want {
		t.Fatalf("unexpected format %q", got)
	}
}

func TestDetectRemoteFormatGGUF(t *testing.T) {
	t.Parallel()

	format, err := DetectRemoteFormat([]string{
		"README.md",
		"deepseek-r1-q4_k_m.gguf",
	})
	if err != nil {
		t.Fatalf("DetectRemoteFormat() error = %v", err)
	}
	if got, want := format, modelsv1alpha1.ModelInputFormatGGUF; got != want {
		t.Fatalf("unexpected format %q", got)
	}
}

func TestDetectPathFormatDirectGGUFFile(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "model.gguf")
	writeTestFile(t, root, "weights")

	format, err := DetectPathFormat(root)
	if err != nil {
		t.Fatalf("DetectPathFormat() error = %v", err)
	}
	if got, want := format, modelsv1alpha1.ModelInputFormatGGUF; got != want {
		t.Fatalf("unexpected format %q", got)
	}
}

func TestDetectPathFormatDirectGGUFMagicWithoutExtension(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "model")
	writeTestFile(t, root, "GGUFweights")

	format, err := DetectPathFormat(root)
	if err != nil {
		t.Fatalf("DetectPathFormat() error = %v", err)
	}
	if got, want := format, modelsv1alpha1.ModelInputFormatGGUF; got != want {
		t.Fatalf("unexpected format %q", got)
	}
}
