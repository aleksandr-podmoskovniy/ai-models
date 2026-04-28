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

func TestInspectModelArchiveBuildsDiffusersSummary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		fileName string
		create   func(string) error
	}{
		{
			name:     "tar",
			fileName: "diffusers.tar",
			create: func(path string) error {
				return createTarArchive(path,
					tarEntry{name: "bundle/model_index.json", content: []byte(`{"_class_name":"TextToVideoSDPipeline"}`)},
					tarEntry{name: "bundle/scheduler/scheduler_config.json", content: []byte(`{}`)},
					tarEntry{name: "bundle/transformer/diffusion_pytorch_model.safetensors", content: []byte("video-weights")},
					tarEntry{name: "bundle/examples/prompt.png", content: []byte("preview")},
				)
			},
		},
		{
			name:     "zip",
			fileName: "diffusers.zip",
			create: func(path string) error {
				return createZipArchive(path,
					tarEntry{name: "bundle/model_index.json", content: []byte(`{"_class_name":"TextToVideoSDPipeline"}`)},
					tarEntry{name: "bundle/scheduler/scheduler_config.json", content: []byte(`{}`)},
					tarEntry{name: "bundle/transformer/diffusion_pytorch_model.safetensors", content: []byte("video-weights")},
					tarEntry{name: "bundle/examples/prompt.png", content: []byte("preview")},
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

			inspection, err := InspectModelArchive(archivePath, "")
			if err != nil {
				t.Fatalf("InspectModelArchive() error = %v", err)
			}
			if got, want := inspection.InputFormat, modelsv1alpha1.ModelInputFormatDiffusers; got != want {
				t.Fatalf("unexpected input format %q", got)
			}
			if got, want := string(inspection.ModelIndexPayload), `{"_class_name":"TextToVideoSDPipeline"}`; got != want {
				t.Fatalf("unexpected model index payload %q", got)
			}
			if got, want := inspection.WeightStats.TotalBytes, int64(len("video-weights")); got != want {
				t.Fatalf("unexpected weight bytes %d", got)
			}
			if len(inspection.SelectedFiles) != 3 {
				t.Fatalf("unexpected selected files %#v", inspection.SelectedFiles)
			}
		})
	}
}
