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

func TestInspectModelArchiveBuildsGGUFSummary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		fileName string
		create   func(string) error
	}{
		{
			name:     "tar",
			fileName: "model.tar",
			create: func(path string) error {
				return createTarArchive(path,
					tarEntry{name: "bundle/deepseek-r1-8b-q4_k_m.gguf", content: []byte("GGUFweights")},
					tarEntry{name: "bundle/README.md", content: []byte("# docs")},
				)
			},
		},
		{
			name:     "zip",
			fileName: "model.zip",
			create: func(path string) error {
				return createZipArchive(path,
					tarEntry{name: "bundle/deepseek-r1-8b-q4_k_m.gguf", content: []byte("GGUFweights")},
					tarEntry{name: "bundle/README.md", content: []byte("# docs")},
				)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			archivePath := filepath.Join(t.TempDir(), tc.fileName)
			if err := tc.create(archivePath); err != nil {
				t.Fatalf("create archive error = %v", err)
			}

			inspection, err := InspectModelArchive(archivePath, modelsv1alpha1.ModelInputFormatGGUF)
			if err != nil {
				t.Fatalf("InspectModelArchive() error = %v", err)
			}
			if got, want := inspection.RootPrefix, "bundle"; got != want {
				t.Fatalf("unexpected root prefix %q", got)
			}
			if got, want := inspection.InputFormat, modelsv1alpha1.ModelInputFormatGGUF; got != want {
				t.Fatalf("unexpected input format %q", got)
			}
			if got, want := inspection.ModelFile, "deepseek-r1-8b-q4_k_m.gguf"; got != want {
				t.Fatalf("unexpected model file %q", got)
			}
			if got, want := inspection.ModelFileSize, int64(len("GGUFweights")); got != want {
				t.Fatalf("unexpected model file size %d", got)
			}
		})
	}
}
