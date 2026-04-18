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
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestInspectTarModelArchiveBuildsSafetensorsSummaryAndRootPrefix(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "model.tar")
	if err := createTarArchive(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
		tarEntry{name: "checkpoint/model.py", content: []byte("print('helper')")},
	); err != nil {
		t.Fatalf("createTarArchive() error = %v", err)
	}

	inspection, err := InspectModelArchive(archivePath, "")
	if err != nil {
		t.Fatalf("InspectModelArchive() error = %v", err)
	}
	if got, want := inspection.RootPrefix, "checkpoint"; got != want {
		t.Fatalf("unexpected root prefix %q", got)
	}
	if got, want := inspection.InputFormat, modelsv1alpha1.ModelInputFormatSafetensors; got != want {
		t.Fatalf("unexpected input format %q", got)
	}
	if len(inspection.SelectedFiles) != 2 {
		t.Fatalf("unexpected selected files %#v", inspection.SelectedFiles)
	}
	if len(inspection.ConfigPayload) == 0 {
		t.Fatal("expected config payload")
	}
	if got, want := inspection.WeightBytes, int64(len("weights")); got != want {
		t.Fatalf("unexpected weight bytes %d", got)
	}
}

func TestInspectTarModelArchiveSupportsGzip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "model.tgz")
	if err := createGzipTarArchive(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
	); err != nil {
		t.Fatalf("createGzipTarArchive() error = %v", err)
	}

	inspection, err := InspectModelArchive(archivePath, modelsv1alpha1.ModelInputFormatSafetensors)
	if err != nil {
		t.Fatalf("InspectModelArchive() error = %v", err)
	}
	if got, want := inspection.RootPrefix, "checkpoint"; got != want {
		t.Fatalf("unexpected root prefix %q", got)
	}
	if got, want := inspection.WeightBytes, int64(len("weights")); got != want {
		t.Fatalf("unexpected weight bytes %d", got)
	}
}

func TestInspectModelArchiveSupportsZip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "model.zip")
	if err := createZipArchive(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
		tarEntry{name: "checkpoint/model.py", content: []byte("print('helper')")},
	); err != nil {
		t.Fatalf("createZipArchive() error = %v", err)
	}

	inspection, err := InspectModelArchive(archivePath, modelsv1alpha1.ModelInputFormatSafetensors)
	if err != nil {
		t.Fatalf("InspectModelArchive() error = %v", err)
	}
	if got, want := inspection.RootPrefix, "checkpoint"; got != want {
		t.Fatalf("unexpected root prefix %q", got)
	}
	if got, want := inspection.InputFormat, modelsv1alpha1.ModelInputFormatSafetensors; got != want {
		t.Fatalf("unexpected input format %q", got)
	}
}
