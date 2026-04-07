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
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func DetectDirFormat(root string) (modelsv1alpha1.ModelInputFormat, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("model input root must not be empty")
	}

	return detectFormat(func(format modelsv1alpha1.ModelInputFormat) error {
		switch format {
		case modelsv1alpha1.ModelInputFormatSafetensors:
			return inspectSafetensorsDir(root)
		case modelsv1alpha1.ModelInputFormatGGUF:
			return inspectGGUFDir(root)
		default:
			return fmt.Errorf("unsupported model input format %q", format)
		}
	})
}

func DetectRemoteFormat(files []string) (modelsv1alpha1.ModelInputFormat, error) {
	return detectFormat(func(format modelsv1alpha1.ModelInputFormat) error {
		_, err := SelectRemoteFiles(format, files)
		return err
	})
}

func detectFormat(match func(modelsv1alpha1.ModelInputFormat) error) (modelsv1alpha1.ModelInputFormat, error) {
	formats := []modelsv1alpha1.ModelInputFormat{
		modelsv1alpha1.ModelInputFormatSafetensors,
		modelsv1alpha1.ModelInputFormatGGUF,
	}

	matches := make([]modelsv1alpha1.ModelInputFormat, 0, len(formats))
	failures := make([]string, 0, len(formats))
	for _, format := range formats {
		err := match(format)
		if err == nil {
			matches = append(matches, format)
			continue
		}
		failures = append(failures, fmt.Sprintf("%s: %v", format, err))
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("failed to determine model input format (%s)", strings.Join(failures, "; "))
	default:
		return "", fmt.Errorf("failed to determine model input format uniquely: matches %s", strings.Join(stringFormats(matches), ", "))
	}
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

func inspectGGUFDir(root string) error {
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
				return fmt.Errorf("input format %q rejects hidden directory %q", modelsv1alpha1.ModelInputFormatGGUF, relative)
			}
			return nil
		}

		action, _, isAsset := classifyGGUFFile(relative)
		if action == fileActionReject {
			return fmt.Errorf("input format %q rejects file %q", modelsv1alpha1.ModelInputFormatGGUF, relative)
		}
		state.hasAsset = state.hasAsset || isAsset
		return nil
	}); err != nil {
		return err
	}

	if !state.hasAsset {
		return fmt.Errorf("input format %q requires at least one .gguf file", modelsv1alpha1.ModelInputFormatGGUF)
	}
	return nil
}

func stringFormats(formats []modelsv1alpha1.ModelInputFormat) []string {
	out := make([]string, 0, len(formats))
	for _, format := range formats {
		out = append(out, string(format))
	}
	return out
}
