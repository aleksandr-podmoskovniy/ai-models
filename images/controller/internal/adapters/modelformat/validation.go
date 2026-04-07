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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

type fileAction int

const (
	fileActionKeep fileAction = iota
	fileActionDrop
	fileActionReject
)

type validationState struct{ hasConfig, hasAsset bool }

func ValidateDir(root string, format modelsv1alpha1.ModelInputFormat) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("model input root must not be empty")
	}
	if strings.TrimSpace(string(format)) == "" {
		return errors.New("model input format must not be empty")
	}

	switch format {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return validateSafetensorsDir(root)
	case modelsv1alpha1.ModelInputFormatGGUF:
		return validateGGUFDir(root)
	default:
		return fmt.Errorf("unsupported model input format %q", format)
	}
}

func SelectRemoteFiles(format modelsv1alpha1.ModelInputFormat, files []string) ([]string, error) {
	if strings.TrimSpace(string(format)) == "" {
		return nil, errors.New("model input format must not be empty")
	}

	switch format {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return selectSafetensorsFiles(files)
	case modelsv1alpha1.ModelInputFormatGGUF:
		return selectGGUFFiles(files)
	default:
		return nil, fmt.Errorf("unsupported model input format %q", format)
	}
}

func validateSafetensorsDir(root string) error {
	state := validationState{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		if entry.IsDir() {
			if shouldDropDirectory(relative) {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
				return filepath.SkipDir
			}
			if strings.HasPrefix(filepath.Base(relative), ".") {
				return fmt.Errorf("input format %q rejects hidden directory %q", modelsv1alpha1.ModelInputFormatSafetensors, relative)
			}
			return nil
		}

		action, isConfig, isAsset := classifySafetensorsFile(relative)
		switch action {
		case fileActionKeep:
			state.hasConfig = state.hasConfig || isConfig
			state.hasAsset = state.hasAsset || isAsset
			return nil
		case fileActionDrop:
			return os.Remove(path)
		default:
			return fmt.Errorf("input format %q rejects file %q", modelsv1alpha1.ModelInputFormatSafetensors, relative)
		}
	}); err != nil {
		return err
	}

	if !state.hasConfig {
		return fmt.Errorf("input format %q requires root config.json", modelsv1alpha1.ModelInputFormatSafetensors)
	}
	if !state.hasAsset {
		return fmt.Errorf("input format %q requires at least one .safetensors file", modelsv1alpha1.ModelInputFormatSafetensors)
	}
	return nil
}

func validateGGUFDir(root string) error {
	state := validationState{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		if entry.IsDir() {
			if shouldDropDirectory(relative) {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
				return filepath.SkipDir
			}
			if strings.HasPrefix(filepath.Base(relative), ".") {
				return fmt.Errorf("input format %q rejects hidden directory %q", modelsv1alpha1.ModelInputFormatGGUF, relative)
			}
			return nil
		}

		action, _, isAsset := classifyGGUFFile(relative)
		switch action {
		case fileActionKeep:
			state.hasAsset = state.hasAsset || isAsset
			return nil
		case fileActionDrop:
			return os.Remove(path)
		default:
			return fmt.Errorf("input format %q rejects file %q", modelsv1alpha1.ModelInputFormatGGUF, relative)
		}
	}); err != nil {
		return err
	}

	if !state.hasAsset {
		return fmt.Errorf("input format %q requires at least one .gguf file", modelsv1alpha1.ModelInputFormatGGUF)
	}
	return nil
}

func selectSafetensorsFiles(files []string) ([]string, error) {
	selected := make([]string, 0, len(files))
	state := validationState{}
	for _, raw := range files {
		relative := normalizeRemotePath(raw)
		if relative == "" {
			continue
		}
		action, isConfig, isAsset := classifySafetensorsFile(relative)
		switch action {
		case fileActionKeep:
			selected = append(selected, relative)
			state.hasConfig = state.hasConfig || isConfig
			state.hasAsset = state.hasAsset || isAsset
		case fileActionDrop:
			continue
		default:
			return nil, fmt.Errorf("input format %q rejects source file %q", modelsv1alpha1.ModelInputFormatSafetensors, relative)
		}
	}
	if !state.hasConfig {
		return nil, fmt.Errorf("input format %q requires root config.json", modelsv1alpha1.ModelInputFormatSafetensors)
	}
	if !state.hasAsset {
		return nil, fmt.Errorf("input format %q requires at least one .safetensors file", modelsv1alpha1.ModelInputFormatSafetensors)
	}
	return selected, nil
}

func selectGGUFFiles(files []string) ([]string, error) {
	selected := make([]string, 0, len(files))
	hasAsset := false
	for _, raw := range files {
		relative := normalizeRemotePath(raw)
		if relative == "" {
			continue
		}
		action, _, isAsset := classifyGGUFFile(relative)
		switch action {
		case fileActionKeep:
			selected = append(selected, relative)
			hasAsset = hasAsset || isAsset
		case fileActionDrop:
			continue
		default:
			return nil, fmt.Errorf("input format %q rejects source file %q", modelsv1alpha1.ModelInputFormatGGUF, relative)
		}
	}
	if !hasAsset {
		return nil, fmt.Errorf("input format %q requires at least one .gguf file", modelsv1alpha1.ModelInputFormatGGUF)
	}
	return selected, nil
}

func classifySafetensorsFile(relative string) (fileAction, bool, bool) {
	base := filepath.Base(relative)
	lowerBase := strings.ToLower(base)
	lowerRelative := strings.ToLower(relative)

	if strings.HasPrefix(base, ".") {
		if base == ".gitattributes" {
			return fileActionDrop, false, false
		}
		return fileActionReject, false, false
	}
	if relative == "config.json" {
		return fileActionKeep, true, false
	}
	if isAllowedMetadataFile(lowerRelative, lowerBase) {
		return fileActionKeep, false, false
	}
	if isBenignExtraFile(lowerBase) {
		return fileActionDrop, false, false
	}
	if hasForbiddenExtension(lowerBase) {
		return fileActionReject, false, false
	}
	if strings.HasSuffix(lowerBase, ".safetensors") {
		return fileActionKeep, false, true
	}
	return fileActionReject, false, false
}

func classifyGGUFFile(relative string) (fileAction, bool, bool) {
	base := filepath.Base(relative)
	lowerBase := strings.ToLower(base)

	if strings.HasPrefix(base, ".") {
		if base == ".gitattributes" {
			return fileActionDrop, false, false
		}
		return fileActionReject, false, false
	}
	if isBenignExtraFile(lowerBase) {
		return fileActionDrop, false, false
	}
	if hasForbiddenExtension(lowerBase) {
		return fileActionReject, false, false
	}
	if strings.HasSuffix(lowerBase, ".gguf") {
		return fileActionKeep, false, true
	}
	return fileActionReject, false, false
}

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
