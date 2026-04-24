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

package sourcefetch

import (
	"errors"
	"os"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
)

func PrepareModelInput(sourcePath, destination string) (string, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return "", errors.New("input source path must not be empty")
	}
	if strings.TrimSpace(destination) == "" {
		return "", errors.New("input destination must not be empty")
	}

	switch {
	case archiveio.IsTarArchive(sourcePath), archiveio.IsZipArchive(sourcePath):
		return UnpackArchive(sourcePath, destination)
	default:
		return materializeSingleFile(sourcePath, destination)
	}
}

func UnpackArchive(archivePath, destination string) (string, error) {
	if strings.TrimSpace(archivePath) == "" {
		return "", errors.New("archive path must not be empty")
	}
	if strings.TrimSpace(destination) == "" {
		return "", errors.New("archive destination must not be empty")
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", err
	}

	switch {
	case archiveio.IsTarArchive(archivePath):
		if err := safeExtractTar(archivePath, destination); err != nil {
			return "", err
		}
	case archiveio.IsZipArchive(archivePath):
		if err := safeExtractZip(archivePath, destination); err != nil {
			return "", err
		}
	default:
		return "", errors.New("archive must be .tar, .tar.gz, .tgz, .tar.zst, .tar.zstd, .tzst, or .zip")
	}

	return normalizeExtractedRoot(destination)
}
