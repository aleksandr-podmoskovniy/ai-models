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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ReadyMarker struct {
	Digest    string
	MediaType string
	ModelPath string
	ReadyAt   time.Time
}

type markerPayload struct {
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType,omitempty"`
	ModelPath string `json:"modelPath,omitempty"`
	ReadyAt   string `json:"readyAt,omitempty"`
}

func MarkerPath(destinationDir string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(destinationDir)), MarkerFileName)
}

func ReadMarker(destinationDir string) (*ReadyMarker, error) {
	body, err := os.ReadFile(MarkerPath(destinationDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var payload markerPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode cache marker: %w", err)
	}
	marker := &ReadyMarker{
		Digest:    strings.TrimSpace(payload.Digest),
		MediaType: strings.TrimSpace(payload.MediaType),
		ModelPath: filepath.Clean(strings.TrimSpace(payload.ModelPath)),
	}
	if raw := strings.TrimSpace(payload.ReadyAt); raw != "" {
		readyAt, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			readyAt, err = time.Parse(time.RFC3339Nano, raw)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode cache marker readyAt: %w", err)
		}
		marker.ReadyAt = readyAt.UTC()
	}
	return marker, nil
}
