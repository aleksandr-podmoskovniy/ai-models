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

package publishplan

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func TestPlanSourceWorker(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		ownerNS string
		spec    modelsv1alpha1.ModelSpec
		wantErr bool
		assert  func(t *testing.T, got SourceWorkerPlan)
	}{
		{
			name: "huggingface source is accepted",
			spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1?revision=main",
				},
				RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"},
			},
			ownerNS: "team-a",
			assert: func(t *testing.T, got SourceWorkerPlan) {
				t.Helper()
				if got.SourceType != modelsv1alpha1.ModelSourceTypeHuggingFace {
					t.Fatalf("unexpected source type %q", got.SourceType)
				}
				if got.HuggingFace == nil || got.HuggingFace.RepoID != "deepseek-ai/DeepSeek-R1" {
					t.Fatalf("unexpected huggingFace plan %#v", got.HuggingFace)
				}
				if got.Task != "text-generation" {
					t.Fatalf("unexpected task %q", got.Task)
				}
			},
		},
		{
			name: "http source requires task and keeps ca bundle",
			spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
				Source: modelsv1alpha1.ModelSourceSpec{
					URL:      "https://downloads.example/model.tar.gz",
					CABundle: []byte("ca-data"),
				},
				RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"},
			},
			ownerNS: "team-a",
			assert: func(t *testing.T, got SourceWorkerPlan) {
				t.Helper()
				if got.HTTP == nil || got.HTTP.URL != "https://downloads.example/model.tar.gz" {
					t.Fatalf("unexpected http plan %#v", got.HTTP)
				}
				if string(got.HTTP.CABundle) != "ca-data" {
					t.Fatalf("unexpected ca bundle %q", string(got.HTTP.CABundle))
				}
			},
		},
		{
			name: "huggingface auth secret resolves owner namespace",
			spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
				Source: modelsv1alpha1.ModelSourceSpec{
					URL:           "https://huggingface.co/deepseek-ai/DeepSeek-R1",
					AuthSecretRef: &modelsv1alpha1.SecretReference{Name: "hf-auth"},
				},
			},
			ownerNS: "team-a",
			assert: func(t *testing.T, got SourceWorkerPlan) {
				t.Helper()
				if got.HuggingFace == nil || got.HuggingFace.AuthSecretRef == nil {
					t.Fatalf("expected resolved huggingFace auth secret, got %#v", got.HuggingFace)
				}
				if got.HuggingFace.AuthSecretRef.Namespace != "team-a" {
					t.Fatalf("unexpected auth namespace %q", got.HuggingFace.AuthSecretRef.Namespace)
				}
			},
		},
		{
			name: "http auth secret keeps explicit namespace",
			spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://downloads.example/model.tar.gz",
					AuthSecretRef: &modelsv1alpha1.SecretReference{
						Namespace: "shared-auth",
						Name:      "http-auth",
					},
				},
				RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"},
			},
			assert: func(t *testing.T, got SourceWorkerPlan) {
				t.Helper()
				if got.HTTP == nil || got.HTTP.AuthSecretRef == nil {
					t.Fatalf("expected resolved http auth secret, got %#v", got.HTTP)
				}
				if got.HTTP.AuthSecretRef.Namespace != "shared-auth" {
					t.Fatalf("unexpected auth namespace %q", got.HTTP.AuthSecretRef.Namespace)
				}
			},
		},
		{
			name: "namespaced source auth secret rejects foreign namespace",
			spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://downloads.example/model.tar.gz",
					AuthSecretRef: &modelsv1alpha1.SecretReference{
						Namespace: "other-team",
						Name:      "http-auth",
					},
				},
				RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"},
			},
			ownerNS: "team-a",
			wantErr: true,
		},
		{
			name: "http without task is rejected",
			spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://downloads.example/model.tar.gz",
				},
			},
			ownerNS: "team-a",
			wantErr: true,
		},
		{
			name: "cluster scoped auth secret requires explicit namespace",
			spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
				Source: modelsv1alpha1.ModelSourceSpec{
					URL:           "https://huggingface.co/deepseek-ai/DeepSeek-R1",
					AuthSecretRef: &modelsv1alpha1.SecretReference{Name: "hf-auth"},
				},
			},
			wantErr: true,
		},
		{
			name: "upload source with staged handle is accepted",
			spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					Upload: &modelsv1alpha1.UploadModelSource{},
				},
				RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"},
			},
			assert: func(t *testing.T, got SourceWorkerPlan) {
				t.Helper()
				if got.Upload == nil || got.Upload.Stage.Key != "uploaded-model-staging/1111-2222/model.gguf" {
					t.Fatalf("unexpected upload plan %#v", got.Upload)
				}
			},
		},
		{
			name: "upload source is rejected on worker path",
			spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					Upload: &modelsv1alpha1.UploadModelSource{},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var uploadStage *cleanuphandle.UploadStagingHandle
			if tc.name == "upload source with staged handle is accepted" {
				uploadStage = &cleanuphandle.UploadStagingHandle{
					Bucket:   "ai-models",
					Key:      "uploaded-model-staging/1111-2222/model.gguf",
					FileName: "model.gguf",
				}
			}
			got, err := PlanSourceWorker(tc.spec, tc.ownerNS, uploadStage)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("PlanSourceWorker() error = %v", err)
			}
			if tc.assert != nil {
				tc.assert(t, got)
			}
		})
	}
}
