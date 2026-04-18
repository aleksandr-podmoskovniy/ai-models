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
	"os"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func ValidateDir(root string, format modelsv1alpha1.ModelInputFormat) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("model input root must not be empty")
	}
	if strings.TrimSpace(string(format)) == "" {
		return errors.New("model input format must not be empty")
	}
	rules, err := rulesForFormat(format)
	if err != nil {
		return err
	}
	return inspectFormatDir(root, rules, true)
}

func ValidatePath(root string, format modelsv1alpha1.ModelInputFormat) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("model input root must not be empty")
	}
	if strings.TrimSpace(string(format)) == "" {
		return errors.New("model input format must not be empty")
	}
	rules, err := rulesForFormat(format)
	if err != nil {
		return err
	}

	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return inspectFormatDir(root, rules, true)
	}
	return inspectFormatFileRoot(root, rules)
}

func SelectRemoteFiles(format modelsv1alpha1.ModelInputFormat, files []string) ([]string, error) {
	if strings.TrimSpace(string(format)) == "" {
		return nil, errors.New("model input format must not be empty")
	}
	rules, err := rulesForFormat(format)
	if err != nil {
		return nil, err
	}
	return selectFormatRemoteFiles(files, rules)
}
