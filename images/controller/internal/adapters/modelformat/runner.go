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

type formatRules struct {
	format               modelsv1alpha1.ModelInputFormat
	classify             func(relative string) (fileAction, bool, bool)
	requiredConfigErrMsg string
	requiredAssetErrMsg  string
}

func rulesForFormat(format modelsv1alpha1.ModelInputFormat) (formatRules, error) {
	switch format {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return safetensorsRules(), nil
	case modelsv1alpha1.ModelInputFormatGGUF:
		return ggufRules(), nil
	default:
		return formatRules{}, fmt.Errorf("unsupported model input format %q", format)
	}
}

func inspectFormatDir(root string, rules formatRules, mutate bool) error {
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
			return handleFormatDir(path, relative, rules, mutate)
		}
		return handleFormatFile(path, relative, rules, mutate, &state)
	}); err != nil {
		return err
	}

	return rules.validateState(state)
}

func selectFormatRemoteFiles(files []string, rules formatRules) ([]string, error) {
	selected := make([]string, 0, len(files))
	state := validationState{}
	for _, raw := range files {
		relative := normalizeRemotePath(raw)
		if relative == "" {
			continue
		}

		action, isConfig, isAsset := rules.classify(relative)
		switch action {
		case fileActionKeep:
			selected = append(selected, relative)
			state.hasConfig = state.hasConfig || isConfig
			state.hasAsset = state.hasAsset || isAsset
		case fileActionDrop:
			continue
		default:
			return nil, fmt.Errorf("input format %q rejects source file %q", rules.format, relative)
		}
	}

	if err := rules.validateState(state); err != nil {
		return nil, err
	}
	return selected, nil
}

func handleFormatDir(path, relative string, rules formatRules, mutate bool) error {
	if shouldDropDirectory(relative) {
		if mutate {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
		return filepath.SkipDir
	}
	if strings.HasPrefix(filepath.Base(relative), ".") {
		return fmt.Errorf("input format %q rejects hidden directory %q", rules.format, relative)
	}
	return nil
}

func handleFormatFile(path, relative string, rules formatRules, mutate bool, state *validationState) error {
	action, isConfig, isAsset := rules.classify(relative)
	switch action {
	case fileActionKeep:
		state.hasConfig = state.hasConfig || isConfig
		state.hasAsset = state.hasAsset || isAsset
		return nil
	case fileActionDrop:
		if mutate {
			return os.Remove(path)
		}
		return nil
	default:
		return fmt.Errorf("input format %q rejects file %q", rules.format, relative)
	}
}

func (rules formatRules) validateState(state validationState) error {
	if rules.requiredConfigErrMsg != "" && !state.hasConfig {
		return fmt.Errorf("input format %q %s", rules.format, rules.requiredConfigErrMsg)
	}
	if rules.requiredAssetErrMsg != "" && !state.hasAsset {
		return fmt.Errorf("input format %q %s", rules.format, rules.requiredAssetErrMsg)
	}
	return nil
}
