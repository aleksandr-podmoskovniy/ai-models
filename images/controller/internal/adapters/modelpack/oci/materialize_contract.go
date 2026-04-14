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

package oci

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func ensureMaterializedModelContract(stagingRoot, resolvedModelPath string) (string, error) {
	contractPath := modelpackports.MaterializedModelPath(stagingRoot)
	switch filepath.Clean(resolvedModelPath) {
	case filepath.Clean(contractPath):
		return contractPath, nil
	case filepath.Clean(stagingRoot):
		return moveMaterializedRootUnderContractPath(stagingRoot, contractPath)
	}

	if _, err := os.Lstat(contractPath); err == nil {
		return "", fmt.Errorf("materialized model contract path %q already exists", contractPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	relativeTarget, err := filepath.Rel(filepath.Dir(contractPath), resolvedModelPath)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(relativeTarget, "..") {
		return "", fmt.Errorf("materialized model path %q escapes contract root %q", resolvedModelPath, stagingRoot)
	}
	if err := os.Symlink(relativeTarget, contractPath); err != nil {
		return "", err
	}

	return contractPath, nil
}

func moveMaterializedRootUnderContractPath(stagingRoot, contractPath string) (string, error) {
	if err := os.MkdirAll(contractPath, 0o755); err != nil {
		return "", err
	}

	entries, err := os.ReadDir(stagingRoot)
	if err != nil {
		return "", err
	}
	contractName := filepath.Base(contractPath)
	for _, entry := range entries {
		if entry.Name() == contractName {
			continue
		}
		sourcePath := filepath.Join(stagingRoot, entry.Name())
		targetPath := filepath.Join(contractPath, entry.Name())
		if err := os.Rename(sourcePath, targetPath); err != nil {
			return "", err
		}
	}

	return contractPath, nil
}
