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

func stringFormats(formats []modelsv1alpha1.ModelInputFormat) []string {
	out := make([]string, 0, len(formats))
	for _, format := range formats {
		out = append(out, string(format))
	}
	return out
}
