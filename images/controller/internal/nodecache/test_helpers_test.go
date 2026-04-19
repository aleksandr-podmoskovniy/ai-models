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
	"os"
	"path/filepath"
	"testing"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func writeReadyMaterialization(t *testing.T, destination, digest string, readyAt time.Time) modelpackports.MaterializeResult {
	t.Helper()

	modelPath := filepath.Join(destination, modelpackports.MaterializedModelPathName)
	if err := os.MkdirAll(modelPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(modelPath) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelPath, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	markerBody, err := json.Marshal(map[string]string{
		"digest":    digest,
		"mediaType": "application/vnd.cncf.model.manifest.v1+json",
		"readyAt":   readyAt.UTC().Format(time.RFC3339Nano),
		"modelPath": modelPath,
	})
	if err != nil {
		t.Fatalf("Marshal(marker) error = %v", err)
	}
	if err := os.WriteFile(MarkerPath(destination), append(markerBody, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}
	return modelpackports.MaterializeResult{
		ModelPath:  modelPath,
		Digest:     digest,
		MediaType:  "application/vnd.cncf.model.manifest.v1+json",
		MarkerPath: MarkerPath(destination),
	}
}
