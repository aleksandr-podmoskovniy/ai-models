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

package publication

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestIsTerminalOperationPhase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   StartPublicationInput
		want    ExecutionMode
		wantErr bool
	}{
		{
			name:  "huggingface uses source worker",
			input: StartPublicationInput{SourceType: modelsv1alpha1.ModelSourceTypeHuggingFace},
			want:  ExecutionModeSourceWorker,
		},
		{
			name:  "http uses source worker",
			input: StartPublicationInput{SourceType: modelsv1alpha1.ModelSourceTypeHTTP},
			want:  ExecutionModeSourceWorker,
		},
		{
			name: "upload uses upload session",
			input: StartPublicationInput{
				SourceType: modelsv1alpha1.ModelSourceTypeUpload,
				Upload: &modelsv1alpha1.UploadModelSource{
					ExpectedFormat: modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory,
				},
				RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"},
			},
			want: ExecutionModeUpload,
		},
		{
			name: "upload modelkit fails",
			input: StartPublicationInput{
				SourceType: modelsv1alpha1.ModelSourceTypeUpload,
				Upload: &modelsv1alpha1.UploadModelSource{
					ExpectedFormat: modelsv1alpha1.ModelUploadFormatModelKit,
				},
				RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"},
			},
			wantErr: true,
		},
		{
			name: "upload without task fails",
			input: StartPublicationInput{
				SourceType: modelsv1alpha1.ModelSourceTypeUpload,
				Upload: &modelsv1alpha1.UploadModelSource{
					ExpectedFormat: modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory,
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported source fails",
			input: StartPublicationInput{
				SourceType: modelsv1alpha1.ModelSourceType("Unsupported"),
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := StartPublication(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("StartPublication() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("StartPublication() = %q, want %q", got, tc.want)
			}
		})
	}
}
