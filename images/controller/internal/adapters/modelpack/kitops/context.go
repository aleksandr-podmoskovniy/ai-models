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

package kitops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func prepareContext(input modelpackports.PublishInput) (string, error) {
	modelDir := strings.TrimSpace(input.ModelDir)
	if modelDir == "" {
		return "", errors.New("model directory must not be empty")
	}

	contextDir, err := os.MkdirTemp("", "ai-model-kitops-kitfile-")
	if err != nil {
		return "", err
	}

	description := strings.TrimSpace(strings.ReplaceAll(input.Description, "\"", "'"))
	if description == "" {
		description = "Published model"
	}
	kitfile := strings.Join([]string{
		"manifestVersion: v1alpha2",
		"package:",
		fmt.Sprintf("  name: %s", packageName(input)),
		fmt.Sprintf("  description: \"%s\"", description),
		"model:",
		"  path: .",
		"",
	}, "\n")

	if err := os.WriteFile(filepath.Join(contextDir, "Kitfile"), []byte(kitfile), 0o644); err != nil {
		os.RemoveAll(contextDir)
		return "", err
	}

	return contextDir, nil
}

func packageName(input modelpackports.PublishInput) string {
	if value := strings.TrimSpace(input.PackageName); value != "" {
		return value
	}
	return packageNameFromOCIReference(input.ArtifactURI)
}
