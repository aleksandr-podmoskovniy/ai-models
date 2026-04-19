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

package nodecache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func LastUsedPath(destinationDir string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(destinationDir)), LastUsedFileName)
}

func TouchUsage(destinationDir string, now time.Time) error {
	destinationDir = filepath.Clean(strings.TrimSpace(destinationDir))
	if destinationDir == "" || destinationDir == "." {
		return errors.New("cache destination directory must not be empty")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(LastUsedPath(destinationDir), []byte(now.UTC().Format(time.RFC3339Nano)+"\n"), 0o644)
}

func ReadLastUsed(destinationDir string) (time.Time, bool, error) {
	body, err := os.ReadFile(LastUsedPath(destinationDir))
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	value := strings.TrimSpace(string(body))
	if value == "" {
		return time.Time{}, false, nil
	}
	lastUsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false, err
	}
	return lastUsed.UTC(), true, nil
}
