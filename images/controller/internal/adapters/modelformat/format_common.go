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
	"slices"
	"strings"
)

type fileAction int

const (
	fileActionKeep fileAction = iota
	fileActionDrop
	fileActionReject
)

type validationState struct{ hasConfig, hasAsset bool }

var droppedDirectoryNames = []string{
	"assets",
	"benchmark",
	"benchmarks",
	"docs",
	"encoding",
	"eval",
	"evaluation",
	"example",
	"examples",
	"figures",
	"images",
	"notebooks",
	"onnx",
	"openvino",
	"coreml",
	"tflite",
	"original",
	"metal",
	"scripts",
	"test",
	"tests",
}

func shouldDropDirectory(relative string) bool {
	base := strings.ToLower(filepath.Base(relative))
	return base == "__macosx" || strings.HasPrefix(base, ".") || slices.Contains(droppedDirectoryNames, base)
}

func normalizeRemotePath(path string) string {
	trimmed := strings.TrimSpace(strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/"))
	for strings.Contains(trimmed, "//") {
		trimmed = strings.ReplaceAll(trimmed, "//", "/")
	}
	return trimmed
}

func hasDroppedPathComponent(relative string) bool {
	for _, part := range strings.Split(strings.ToLower(filepath.ToSlash(relative)), "/") {
		if part == "" {
			continue
		}
		if part == "__macosx" || strings.HasPrefix(part, ".") || slices.Contains(droppedDirectoryNames, part) {
			return true
		}
	}
	return false
}

func isModelCompanionFile(lowerRelative, lowerBase string) bool {
	if slices.Contains([]string{
		"generation_config.json",
		"tokenizer.json",
		"tokenizer_config.json",
		"special_tokens_map.json",
		"preprocessor_config.json",
		"processor_config.json",
		"added_tokens.json",
		"vocab.json",
		"vocab.txt",
		"merges.txt",
		"tokenizer.model",
		"spiece.model",
		"sentencepiece.bpe.model",
		"chat_template.jinja",
		"modules.json",
		"dtypes.json",
		"params",
		"params.json",
	}, lowerRelative) {
		return true
	}
	if strings.HasSuffix(lowerBase, ".index.json") {
		return true
	}
	if !hasSuffix(lowerBase, ".json", ".yaml", ".yml", ".xml", ".jinja", ".model", ".txt") {
		return false
	}
	for _, token := range []string{
		"config",
		"tokenizer",
		"vocab",
		"merge",
		"special_tokens",
		"added_tokens",
		"preprocessor",
		"processor",
		"chat_template",
		"module",
		"pooling",
		"params",
		"dtype",
		"template",
	} {
		if strings.Contains(lowerBase, token) {
			return true
		}
	}
	return false
}

func isAlternativeExportArtifact(lowerRelative, lowerBase string) bool {
	for _, prefix := range []string{
		"onnx/",
		"openvino/",
		"coreml/",
		"tflite/",
		"original/",
		"metal/",
	} {
		if strings.HasPrefix(lowerRelative, prefix) {
			return true
		}
	}

	if slices.Contains([]string{
		"pytorch_model.bin",
		"tf_model.h5",
		"flax_model.msgpack",
		"rust_model.ot",
		"model.ckpt.index",
		"model.bin",
	}, lowerBase) {
		return true
	}

	if strings.HasPrefix(lowerBase, "pytorch_model") && strings.HasSuffix(lowerBase, ".bin") {
		return true
	}
	if strings.HasPrefix(lowerBase, "model.ckpt.") || strings.HasPrefix(lowerBase, "model.ckpt-") {
		return true
	}

	return hasSuffix(lowerBase,
		".onnx",
		".tflite",
		".mlmodel",
		".pdmodel",
		".pdiparams",
		".pt",
		".pth",
		".h5",
		".msgpack",
		".ot",
		".imatrix",
		".gguf_file",
	)
}

func isBenignExtraFile(lowerBase string) bool {
	if strings.HasPrefix(lowerBase, "readme") || strings.HasPrefix(lowerBase, "license") || strings.HasPrefix(lowerBase, "notice") || strings.Contains(lowerBase, "usage_policy") || lowerBase == "use_policy" {
		return true
	}
	return hasSuffix(lowerBase, ".md", ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".pdf", ".txt", ".jsonl", ".csv")
}

func hasDroppedScriptExtension(lowerBase string) bool {
	return hasSuffix(lowerBase,
		".py",
		".sh",
		".bash",
		".zsh",
		".bat",
		".cmd",
		".php",
		".pl",
		".rb",
		".ps1",
	)
}

func hasHardRejectExtension(lowerBase string) bool {
	return hasSuffix(lowerBase,
		".so",
		".dll",
		".dylib",
		".exe",
		".jar",
		".class",
		".wasm",
		".zip",
		".tar",
		".tgz",
		".gz",
		".bz2",
		".xz",
		".7z",
		".rar",
	)
}

func hasSuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}
