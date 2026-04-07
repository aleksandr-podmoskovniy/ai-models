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
	"os"
	"path/filepath"
	"slices"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestValidateDirSafetensors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.json"), `{"model_type":"qwen3"}`)
	writeTestFile(t, filepath.Join(root, "model.safetensors"), "weights")
	writeTestFile(t, filepath.Join(root, "README.md"), "# docs")

	if err := ValidateDir(root, modelsv1alpha1.ModelInputFormatSafetensors); err != nil {
		t.Fatalf("ValidateDir() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("expected README.md to be removed, stat err = %v", err)
	}
}

func TestValidateDirGGUF(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "model-q4_k_m.gguf"), "weights")

	if err := ValidateDir(root, modelsv1alpha1.ModelInputFormatGGUF); err != nil {
		t.Fatalf("ValidateDir() error = %v", err)
	}
}

func TestValidateDirRejectsForbiddenFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.json"), `{"model_type":"qwen3"}`)
	writeTestFile(t, filepath.Join(root, "model.safetensors"), "weights")
	writeTestFile(t, filepath.Join(root, "model.py"), "print('boom')")

	if err := ValidateDir(root, modelsv1alpha1.ModelInputFormatSafetensors); err == nil {
		t.Fatal("expected forbidden file validation error")
	}
}

func TestValidateDirRequiresExpectedAssets(t *testing.T) {
	t.Parallel()

	missingConfig := t.TempDir()
	writeTestFile(t, filepath.Join(missingConfig, "model.safetensors"), "weights")
	if err := ValidateDir(missingConfig, modelsv1alpha1.ModelInputFormatSafetensors); err == nil {
		t.Fatal("expected safetensors config validation error")
	}

	missingGGUF := t.TempDir()
	writeTestFile(t, filepath.Join(missingGGUF, "README.md"), "docs")
	if err := ValidateDir(missingGGUF, modelsv1alpha1.ModelInputFormatGGUF); err == nil {
		t.Fatal("expected gguf asset validation error")
	}
}

func TestSelectRemoteFiles(t *testing.T) {
	t.Parallel()

	selected, err := SelectRemoteFiles(modelsv1alpha1.ModelInputFormatSafetensors, []string{
		"README.md",
		"config.json",
		"model-00001-of-00002.safetensors",
		"tokenizer.json",
	})
	if err != nil {
		t.Fatalf("SelectRemoteFiles() error = %v", err)
	}
	if !slices.Equal(selected, []string{
		"config.json",
		"model-00001-of-00002.safetensors",
		"tokenizer.json",
	}) {
		t.Fatalf("unexpected selected files %#v", selected)
	}

	selected, err = SelectRemoteFiles(modelsv1alpha1.ModelInputFormatGGUF, []string{
		"README.md",
		"deepseek-r1-q4_k_m.gguf",
	})
	if err != nil {
		t.Fatalf("SelectRemoteFiles(GGUF) error = %v", err)
	}
	if !slices.Equal(selected, []string{"deepseek-r1-q4_k_m.gguf"}) {
		t.Fatalf("unexpected gguf selected files %#v", selected)
	}
}

func TestSelectRemoteFilesRejectsRemoteCode(t *testing.T) {
	t.Parallel()

	_, err := SelectRemoteFiles(modelsv1alpha1.ModelInputFormatSafetensors, []string{
		"config.json",
		"model.safetensors",
		"modeling_qwen.py",
	})
	if err == nil {
		t.Fatal("expected remote code validation error")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
