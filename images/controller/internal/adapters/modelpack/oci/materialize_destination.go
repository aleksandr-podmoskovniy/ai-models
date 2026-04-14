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
	"os"
)

func replaceMaterializedDestination(stagingRoot, destination string) error {
	parent := materializationParent(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}

	backupDir, hadExisting, err := stageExistingDestination(destination)
	if err != nil {
		return err
	}

	if err := os.Rename(stagingRoot, destination); err != nil {
		if hadExisting {
			_ = os.Rename(backupDir, destination)
		}
		return err
	}
	if hadExisting {
		return os.RemoveAll(backupDir)
	}
	return nil
}

func stageExistingDestination(destination string) (string, bool, error) {
	if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	backupDir := destination + ".previous"
	if err := os.RemoveAll(backupDir); err != nil {
		return "", false, err
	}
	if err := os.Rename(destination, backupDir); err != nil {
		return "", false, err
	}
	return backupDir, true, nil
}
