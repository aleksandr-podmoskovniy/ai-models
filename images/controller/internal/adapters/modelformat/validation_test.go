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
	writeTestFile(t, filepath.Join(root, "chat_template.json"), `{"template":"..."}`)
	writeTestFile(t, filepath.Join(root, "1_Pooling", "config.json"), `{"pooling_mode":"cls"}`)
	writeTestFile(t, filepath.Join(root, "README.md"), "# docs")
	writeTestFile(t, filepath.Join(root, "onnx", "model.onnx"), "onnx")
	writeTestFile(t, filepath.Join(root, "pytorch_model.bin"), "weights")
	writeTestFile(t, filepath.Join(root, "modeling_qwen.py"), "print('helper')")
	writeTestFile(t, filepath.Join(root, ".eval_results", "result.yaml"), "score: 1")

	if err := ValidateDir(root, modelsv1alpha1.ModelInputFormatSafetensors); err != nil {
		t.Fatalf("ValidateDir() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "chat_template.json")); err != nil {
		t.Fatalf("expected chat_template.json to remain, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "1_Pooling", "config.json")); err != nil {
		t.Fatalf("expected 1_Pooling/config.json to remain, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("expected README.md to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "onnx", "model.onnx")); !os.IsNotExist(err) {
		t.Fatalf("expected onnx/model.onnx to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "pytorch_model.bin")); !os.IsNotExist(err) {
		t.Fatalf("expected pytorch_model.bin to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "modeling_qwen.py")); !os.IsNotExist(err) {
		t.Fatalf("expected modeling_qwen.py to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".eval_results")); !os.IsNotExist(err) {
		t.Fatalf("expected .eval_results to be removed, stat err = %v", err)
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

func TestValidatePathGGUFFile(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "model.gguf")
	writeTestFile(t, root, "weights")

	if err := ValidatePath(root, modelsv1alpha1.ModelInputFormatGGUF); err != nil {
		t.Fatalf("ValidatePath() error = %v", err)
	}
}

func TestValidatePathSafetensorsFileRequiresConfig(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "model.safetensors")
	writeTestFile(t, root, "weights")

	if err := ValidatePath(root, modelsv1alpha1.ModelInputFormatSafetensors); err == nil {
		t.Fatal("expected safetensors file-only validation error")
	}
}

func TestValidateDirRejectsHardRejectPayload(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.json"), `{"model_type":"qwen3"}`)
	writeTestFile(t, filepath.Join(root, "model.safetensors"), "weights")
	writeTestFile(t, filepath.Join(root, "libpayload.so"), "boom")

	if err := ValidateDir(root, modelsv1alpha1.ModelInputFormatSafetensors); err == nil {
		t.Fatal("expected hard-reject payload validation error")
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
		"chat_template.json",
		"config_sentence_transformers.json",
		"1_Pooling/config.json",
		"modules.json",
		"video_preprocessor_config.json",
		"model-00001-of-00002.safetensors",
		"onnx/model.onnx",
		"original/params.json",
		"pytorch_model.bin",
		"modeling_phi3.py",
		"tokenizer.json",
	})
	if err != nil {
		t.Fatalf("SelectRemoteFiles() error = %v", err)
	}
	if !slices.Equal(selected, []string{
		"config.json",
		"chat_template.json",
		"config_sentence_transformers.json",
		"1_Pooling/config.json",
		"modules.json",
		"video_preprocessor_config.json",
		"model-00001-of-00002.safetensors",
		"tokenizer.json",
	}) {
		t.Fatalf("unexpected selected files %#v", selected)
	}

	selected, err = SelectRemoteFiles(modelsv1alpha1.ModelInputFormatGGUF, []string{
		"README.md",
		"params",
		"tokenizer_config.json",
		"imatrix_unsloth.gguf_file",
		"deepseek-r1-q4_k_m.gguf",
	})
	if err != nil {
		t.Fatalf("SelectRemoteFiles(GGUF) error = %v", err)
	}
	if !slices.Equal(selected, []string{"params", "tokenizer_config.json", "deepseek-r1-q4_k_m.gguf"}) {
		t.Fatalf("unexpected gguf selected files %#v", selected)
	}
}

func TestSelectRemoteFilesDiffusersLayout(t *testing.T) {
	t.Parallel()

	selected, err := SelectRemoteFiles(modelsv1alpha1.ModelInputFormatDiffusers, []string{
		"README.md",
		"model_index.json",
		"scheduler/scheduler_config.json",
		"text_encoder/config.json",
		"text_encoder/pytorch_model.bin",
		"unet/config.json",
		"unet/diffusion_pytorch_model.bin",
		"vae/config.json",
		"vae/diffusion_pytorch_model.safetensors",
		"onnx/model.onnx",
		"examples/prompt.png",
	})
	if err != nil {
		t.Fatalf("SelectRemoteFiles() error = %v", err)
	}
	if !slices.Equal(selected, []string{
		"model_index.json",
		"scheduler/scheduler_config.json",
		"text_encoder/config.json",
		"text_encoder/pytorch_model.bin",
		"unet/config.json",
		"unet/diffusion_pytorch_model.bin",
		"vae/config.json",
		"vae/diffusion_pytorch_model.safetensors",
	}) {
		t.Fatalf("unexpected selected files %#v", selected)
	}
}

func TestSelectRemoteFilesSafetensorsRejectsDiffusersLayout(t *testing.T) {
	t.Parallel()

	_, err := SelectRemoteFiles(modelsv1alpha1.ModelInputFormatSafetensors, []string{
		"model_index.json",
		"unet/diffusion_pytorch_model.safetensors",
	})
	if err == nil {
		t.Fatal("expected safetensors validation error for diffusers layout")
	}
}

func TestSelectRemoteFilesDropsHelperScripts(t *testing.T) {
	t.Parallel()

	selected, err := SelectRemoteFiles(modelsv1alpha1.ModelInputFormatSafetensors, []string{
		"config.json",
		"model.safetensors",
		"modeling_qwen.py",
		"sample_finetune.py",
	})
	if err != nil {
		t.Fatalf("SelectRemoteFiles() error = %v", err)
	}
	if !slices.Equal(selected, []string{"config.json", "model.safetensors"}) {
		t.Fatalf("unexpected selected files %#v", selected)
	}
}

func TestSelectRemoteFilesRejectsCompiledPayload(t *testing.T) {
	t.Parallel()

	_, err := SelectRemoteFiles(modelsv1alpha1.ModelInputFormatSafetensors, []string{
		"config.json",
		"model.safetensors",
		"libpayload.so",
	})
	if err == nil {
		t.Fatal("expected compiled payload validation error")
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
