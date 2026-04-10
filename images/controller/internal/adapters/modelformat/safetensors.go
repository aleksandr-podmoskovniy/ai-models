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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

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

func inspectSafetensorsDir(root string) error {
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
				return filepath.SkipDir
			}
			if strings.HasPrefix(filepath.Base(relative), ".") {
				return fmt.Errorf("input format %q rejects hidden directory %q", modelsv1alpha1.ModelInputFormatSafetensors, relative)
			}
			return nil
		}

		action, isConfig, isAsset := classifySafetensorsFile(relative)
		if action == fileActionReject {
			return fmt.Errorf("input format %q rejects file %q", modelsv1alpha1.ModelInputFormatSafetensors, relative)
		}
		state.hasConfig = state.hasConfig || isConfig
		state.hasAsset = state.hasAsset || isAsset
		return nil
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
