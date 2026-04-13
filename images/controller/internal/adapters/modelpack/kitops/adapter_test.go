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
	"os"
	"path/filepath"
	"strings"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestRegistryFromOCIReference(t *testing.T) {
	t.Parallel()

	if got, want := registryFromOCIReference("registry.example.com/ai-models/catalog/model:published"), "registry.example.com"; got != want {
		t.Fatalf("unexpected registry %q", got)
	}
}

func TestImmutableOCIReference(t *testing.T) {
	t.Parallel()

	if got, want := immutableOCIReference("registry.example.com/ai-models/catalog/model:published", "sha256:deadbeef"), "registry.example.com/ai-models/catalog/model@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected immutable reference %q", got)
	}
}

func TestPrepareContextWritesKitfileForDirectModelDir(t *testing.T) {
	t.Parallel()

	modelDir := filepath.Join(t.TempDir(), "checkpoint")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "config.json"), []byte(`{"model_type":"gemma4"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}

	contextDir, err := prepareContext(modelpackports.PublishInput{
		ModelDir:    modelDir,
		ArtifactURI: "registry.example.com/ns/model:published",
		Description: `desc "quoted"`,
	})
	if err != nil {
		t.Fatalf("prepareContext() error = %v", err)
	}
	defer os.RemoveAll(contextDir)

	if _, err := os.Lstat(filepath.Join(contextDir, "model")); !os.IsNotExist(err) {
		t.Fatalf("prepareContext() must not create model symlink, stat err = %v", err)
	}

	kitfilePath := filepath.Join(contextDir, "Kitfile")
	payload, err := os.ReadFile(kitfilePath)
	if err != nil {
		t.Fatalf("ReadFile(Kitfile) error = %v", err)
	}
	text := string(payload)
	if !strings.Contains(text, "  path: .") {
		t.Fatalf("Kitfile must pack the model directory directly, got %q", text)
	}
	if !strings.Contains(text, `description: "desc 'quoted'"`) {
		t.Fatalf("Kitfile must sanitize quotes, got %q", text)
	}
}
