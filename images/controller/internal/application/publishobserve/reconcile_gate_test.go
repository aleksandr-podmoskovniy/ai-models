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

package publishobserve

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
)

func TestDecideCatalogStatusReconcile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   CatalogStatusReconcileInput
		assert  func(t *testing.T, got CatalogStatusReconcileDecision)
		wantErr bool
	}{
		{
			name: "deleting object skips reconcile",
			input: CatalogStatusReconcileInput{
				Deleting: true,
			},
			assert: func(t *testing.T, got CatalogStatusReconcileDecision) {
				t.Helper()
				if !got.Skip {
					t.Fatal("expected skip")
				}
			},
		},
		{
			name: "ready object with cleanup handle skips reconcile",
			input: CatalogStatusReconcileInput{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
				},
				Current: modelsv1alpha1.ModelStatus{
					ObservedGeneration: 7,
					Phase:              modelsv1alpha1.ModelPhaseReady,
				},
				Generation:       7,
				HasCleanupHandle: true,
			},
			assert: func(t *testing.T, got CatalogStatusReconcileDecision) {
				t.Helper()
				if !got.Skip || got.SourceType != modelsv1alpha1.ModelSourceTypeHuggingFace {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "failed object skips reconcile",
			input: CatalogStatusReconcileInput{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
				},
				Current: modelsv1alpha1.ModelStatus{
					ObservedGeneration: 7,
					Phase:              modelsv1alpha1.ModelPhaseFailed,
				},
				Generation: 7,
			},
			assert: func(t *testing.T, got CatalogStatusReconcileDecision) {
				t.Helper()
				if !got.Skip {
					t.Fatal("expected skip")
				}
			},
		},
		{
			name: "remote source uses source worker",
			input: CatalogStatusReconcileInput{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
				},
			},
			assert: func(t *testing.T, got CatalogStatusReconcileDecision) {
				t.Helper()
				if got.Skip || got.Mode != publicationapp.ExecutionModeSourceWorker {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload source uses upload session",
			input: CatalogStatusReconcileInput{
				Source: modelsv1alpha1.ModelSourceSpec{
					Upload: &modelsv1alpha1.UploadModelSource{},
				},
			},
			assert: func(t *testing.T, got CatalogStatusReconcileDecision) {
				t.Helper()
				if got.Skip || got.Mode != publicationapp.ExecutionModeUpload || got.SourceType != modelsv1alpha1.ModelSourceTypeUpload {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload source keeps source worker while publishing cleanup handoff is pending",
			input: CatalogStatusReconcileInput{
				Source: modelsv1alpha1.ModelSourceSpec{
					Upload: &modelsv1alpha1.UploadModelSource{},
				},
				Current: modelsv1alpha1.ModelStatus{
					ObservedGeneration: 7,
					Phase:              modelsv1alpha1.ModelPhasePublishing,
				},
				Generation:       7,
				HasCleanupHandle: true,
			},
			assert: func(t *testing.T, got CatalogStatusReconcileDecision) {
				t.Helper()
				if got.Skip || got.Mode != publicationapp.ExecutionModeSourceWorker || got.SourceType != modelsv1alpha1.ModelSourceTypeUpload {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "upload without task still uses upload session",
			input: CatalogStatusReconcileInput{
				Source: modelsv1alpha1.ModelSourceSpec{
					Upload: &modelsv1alpha1.UploadModelSource{},
				},
			},
			assert: func(t *testing.T, got CatalogStatusReconcileDecision) {
				t.Helper()
				if got.Skip || got.Mode != publicationapp.ExecutionModeUpload || got.SourceType != modelsv1alpha1.ModelSourceTypeUpload {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecideCatalogStatusReconcile(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("DecideCatalogStatusReconcile() error = %v", err)
			}
			tc.assert(t, got)
		})
	}
}
