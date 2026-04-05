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
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publication"
)

func TestIssueUploadSession(t *testing.T) {
	t.Parallel()

	size := int64(128)
	cases := []struct {
		name    string
		input   UploadSessionIssueRequest
		assert  func(t *testing.T, got UploadSessionPlan)
		wantErr bool
	}{
		{
			name: "accepted upload returns plan",
			input: UploadSessionIssueRequest{
				OwnerUID:           "1111-2222",
				OwnerKind:          modelsv1alpha1.ModelKind,
				OwnerName:          "deepseek-r1",
				Identity:           publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				OperationName:      "op-a",
				OperationNamespace: "d8-ai-models",
				SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
				Upload: &modelsv1alpha1.UploadModelSource{
					ExpectedFormat:    modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory,
					ExpectedSizeBytes: &size,
				},
				Task: " text-generation ",
			},
			assert: func(t *testing.T, got UploadSessionPlan) {
				t.Helper()
				if got.ExpectedFormat != modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory {
					t.Fatalf("unexpected format %#v", got)
				}
				if got.ExpectedSizeBytes == nil || *got.ExpectedSizeBytes != size {
					t.Fatalf("unexpected size %#v", got)
				}
				if got.Task != "text-generation" {
					t.Fatalf("unexpected task %#v", got)
				}
			},
		},
		{
			name: "missing owner uid fails closed",
			input: UploadSessionIssueRequest{
				OwnerKind:          modelsv1alpha1.ModelKind,
				OwnerName:          "deepseek-r1",
				Identity:           publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				OperationName:      "op-a",
				OperationNamespace: "d8-ai-models",
				SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
				Upload: &modelsv1alpha1.UploadModelSource{
					ExpectedFormat: modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory,
				},
				Task: "text-generation",
			},
			wantErr: true,
		},
		{
			name: "non-upload source is rejected",
			input: UploadSessionIssueRequest{
				OwnerUID:           "1111-2222",
				OwnerKind:          modelsv1alpha1.ModelKind,
				OwnerName:          "deepseek-r1",
				Identity:           publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				OperationName:      "op-a",
				OperationNamespace: "d8-ai-models",
				SourceType:         modelsv1alpha1.ModelSourceTypeHuggingFace,
				Task:               "text-generation",
			},
			wantErr: true,
		},
		{
			name: "missing upload source is rejected",
			input: UploadSessionIssueRequest{
				OwnerUID:           "1111-2222",
				OwnerKind:          modelsv1alpha1.ModelKind,
				OwnerName:          "deepseek-r1",
				Identity:           publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				OperationName:      "op-a",
				OperationNamespace: "d8-ai-models",
				SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
				Task:               "text-generation",
			},
			wantErr: true,
		},
		{
			name: "modelkit is rejected on current path",
			input: UploadSessionIssueRequest{
				OwnerUID:           "1111-2222",
				OwnerKind:          modelsv1alpha1.ModelKind,
				OwnerName:          "deepseek-r1",
				Identity:           publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				OperationName:      "op-a",
				OperationNamespace: "d8-ai-models",
				SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
				Upload: &modelsv1alpha1.UploadModelSource{
					ExpectedFormat: modelsv1alpha1.ModelUploadFormatModelKit,
				},
				Task: "text-generation",
			},
			wantErr: true,
		},
		{
			name: "missing task is rejected",
			input: UploadSessionIssueRequest{
				OwnerUID:           "1111-2222",
				OwnerKind:          modelsv1alpha1.ModelKind,
				OwnerName:          "deepseek-r1",
				Identity:           publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				OperationName:      "op-a",
				OperationNamespace: "d8-ai-models",
				SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
				Upload: &modelsv1alpha1.UploadModelSource{
					ExpectedFormat: modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory,
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := IssueUploadSession(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("IssueUploadSession() error = %v", err)
			}
			tc.assert(t, got)
		})
	}
}
