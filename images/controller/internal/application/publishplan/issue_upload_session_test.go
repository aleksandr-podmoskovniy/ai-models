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
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
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
			name: "accepted upload returns expected size plan",
			input: UploadSessionIssueRequest{
				OwnerUID:  "1111-2222",
				OwnerKind: modelsv1alpha1.ModelKind,
				OwnerName: "deepseek-r1",
				Identity:  publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				Source:    modelsv1alpha1.ModelSourceSpec{Upload: &modelsv1alpha1.UploadModelSource{ExpectedSizeBytes: &size}},
			},
			assert: func(t *testing.T, got UploadSessionPlan) {
				t.Helper()
				if got.ExpectedSizeBytes == nil || *got.ExpectedSizeBytes != size {
					t.Fatalf("unexpected size %#v", got)
				}
			},
		},
		{
			name: "accepted upload without expected size returns empty size plan",
			input: UploadSessionIssueRequest{
				OwnerUID:  "1111-2222",
				OwnerKind: modelsv1alpha1.ModelKind,
				OwnerName: "deepseek-r1",
				Identity:  publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				Source:    modelsv1alpha1.ModelSourceSpec{Upload: &modelsv1alpha1.UploadModelSource{}},
			},
			assert: func(t *testing.T, got UploadSessionPlan) {
				t.Helper()
				if got.ExpectedSizeBytes != nil {
					t.Fatalf("unexpected size %#v", got)
				}
			},
		},
		{
			name: "missing owner uid fails closed",
			input: UploadSessionIssueRequest{
				OwnerKind: modelsv1alpha1.ModelKind,
				OwnerName: "deepseek-r1",
				Identity:  publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				Source:    modelsv1alpha1.ModelSourceSpec{Upload: &modelsv1alpha1.UploadModelSource{}},
			},
			wantErr: true,
		},
		{
			name: "missing owner kind fails closed",
			input: UploadSessionIssueRequest{
				OwnerUID:  "1111-2222",
				OwnerName: "deepseek-r1",
				Identity:  publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				Source:    modelsv1alpha1.ModelSourceSpec{Upload: &modelsv1alpha1.UploadModelSource{}},
			},
			wantErr: true,
		},
		{
			name: "missing owner name fails closed",
			input: UploadSessionIssueRequest{
				OwnerUID:  "1111-2222",
				OwnerKind: modelsv1alpha1.ModelKind,
				Identity:  publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				Source:    modelsv1alpha1.ModelSourceSpec{Upload: &modelsv1alpha1.UploadModelSource{}},
			},
			wantErr: true,
		},
		{
			name: "invalid identity fails closed",
			input: UploadSessionIssueRequest{
				OwnerUID:  "1111-2222",
				OwnerKind: modelsv1alpha1.ModelKind,
				OwnerName: "deepseek-r1",
				Identity:  publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Name: "deepseek-r1"},
				Source:    modelsv1alpha1.ModelSourceSpec{Upload: &modelsv1alpha1.UploadModelSource{}},
			},
			wantErr: true,
		},
		{
			name: "non-upload source is rejected",
			input: UploadSessionIssueRequest{
				OwnerUID:  "1111-2222",
				OwnerKind: modelsv1alpha1.ModelKind,
				OwnerName: "deepseek-r1",
				Identity:  publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
				Source:    modelsv1alpha1.ModelSourceSpec{URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1"},
			},
			wantErr: true,
		},
		{
			name: "missing upload source is rejected",
			input: UploadSessionIssueRequest{
				OwnerUID:  "1111-2222",
				OwnerKind: modelsv1alpha1.ModelKind,
				OwnerName: "deepseek-r1",
				Identity:  publicationdata.Identity{Scope: publicationdata.ScopeNamespaced, Namespace: "team-a", Name: "deepseek-r1"},
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
