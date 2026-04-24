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

package catalogstatus

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestDecideCatalogStatusReconcileBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		deleting   bool
		source     modelsv1alpha1.ModelSourceSpec
		stage      bool
		current    modelsv1alpha1.ModelStatus
		generation int64
		handle     bool
		wantSkip   bool
		wantMode   runtimeMode
		wantErr    bool
	}{
		{
			name:     "deleting skips runtime",
			deleting: true,
			source:   huggingFaceSource(),
			wantSkip: true,
		},
		{
			name:       "ready with cleanup handle skips replay",
			source:     huggingFaceSource(),
			current:    modelsv1alpha1.ModelStatus{ObservedGeneration: 7, Phase: modelsv1alpha1.ModelPhaseReady},
			generation: 7,
			handle:     true,
			wantSkip:   true,
		},
		{
			name:       "failed observed generation skips replay",
			source:     huggingFaceSource(),
			current:    modelsv1alpha1.ModelStatus{ObservedGeneration: 7, Phase: modelsv1alpha1.ModelPhaseFailed},
			generation: 7,
			wantSkip:   true,
		},
		{
			name:     "huggingface uses source worker",
			source:   huggingFaceSource(),
			wantMode: runtimeModeSourceWorker,
		},
		{
			name:     "new upload uses upload session",
			source:   uploadSource(),
			wantMode: runtimeModeUpload,
		},
		{
			name:     "staged upload uses source worker",
			source:   uploadSource(),
			stage:    true,
			wantMode: runtimeModeSourceWorker,
		},
		{
			name:       "publishing staged upload keeps source worker",
			source:     uploadSource(),
			current:    modelsv1alpha1.ModelStatus{ObservedGeneration: 7, Phase: modelsv1alpha1.ModelPhasePublishing},
			generation: 7,
			handle:     true,
			wantMode:   runtimeModeSourceWorker,
		},
		{
			name:    "missing source fails closed",
			source:  modelsv1alpha1.ModelSourceSpec{},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := decideCatalogStatusReconcile(
				tc.deleting,
				tc.source,
				tc.stage,
				tc.current,
				tc.generation,
				tc.handle,
			)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decideCatalogStatusReconcile() error = %v", err)
			}
			if got.Skip != tc.wantSkip {
				t.Fatalf("Skip = %v, want %v", got.Skip, tc.wantSkip)
			}
			if !tc.wantSkip && got.Mode != tc.wantMode {
				t.Fatalf("Mode = %q, want %q", got.Mode, tc.wantMode)
			}
		})
	}
}

func huggingFaceSource() modelsv1alpha1.ModelSourceSpec {
	return modelsv1alpha1.ModelSourceSpec{URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1"}
}

func uploadSource() modelsv1alpha1.ModelSourceSpec {
	return modelsv1alpha1.ModelSourceSpec{Upload: &modelsv1alpha1.UploadModelSource{}}
}
