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

func shouldDropDirectory(relative string) bool {
	return filepath.Base(relative) == "__MACOSX"
}

func normalizeRemotePath(path string) string {
	trimmed := strings.TrimSpace(strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/"))
	for strings.Contains(trimmed, "//") {
		trimmed = strings.ReplaceAll(trimmed, "//", "/")
	}
	return trimmed
}

func isAllowedMetadataFile(lowerRelative, lowerBase string) bool {
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
	}, lowerRelative) {
		return true
	}
	return strings.HasSuffix(lowerBase, ".index.json")
}

func isAlternativeExportArtifact(lowerRelative, lowerBase string) bool {
	for _, prefix := range []string{
		"onnx/",
		"openvino/",
		"coreml/",
		"tflite/",
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
	}, lowerBase) {
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
	)
}

func isBenignExtraFile(lowerBase string) bool {
	if strings.HasPrefix(lowerBase, "readme") || strings.HasPrefix(lowerBase, "license") || strings.HasPrefix(lowerBase, "notice") {
		return true
	}
	return hasSuffix(lowerBase, ".md", ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg")
}

func hasForbiddenExtension(lowerBase string) bool {
	return hasSuffix(lowerBase,
		".py",
		".sh",
		".bash",
		".zsh",
		".so",
		".dll",
		".dylib",
		".exe",
		".bat",
		".cmd",
		".jar",
		".class",
		".php",
		".pl",
		".rb",
		".ps1",
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
