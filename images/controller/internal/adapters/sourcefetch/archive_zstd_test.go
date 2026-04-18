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
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestInspectModelArchiveSupportsZstdTar(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "model.tar.zst")
	if err := createZstdTarArchive(archivePath,
		tarEntry{name: "checkpoint/config.json", content: []byte(`{"architectures":["LlamaForCausalLM"]}`)},
		tarEntry{name: "checkpoint/model.safetensors", content: []byte("weights")},
	); err != nil {
		t.Fatalf("createZstdTarArchive() error = %v", err)
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

func createZstdTarArchive(path string, entries ...tarEntry) error {
	var buffer bytes.Buffer
	encoder, err := zstd.NewWriter(&buffer)
	if err != nil {
		return err
	}
	writer := tar.NewWriter(encoder)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.content))}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if _, err := writer.Write(entry.content); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}
