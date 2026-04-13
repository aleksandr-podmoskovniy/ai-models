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

package publishop

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"k8s.io/apimachinery/pkg/types"
)

func TestRequestValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   Request
		wantErr bool
	}{
		{
			name: "valid huggingface request",
			input: Request{
				Owner:    validOwner(),
				Identity: validIdentity(),
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{
						URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
					},
				},
			},
		},
		{
			name: "missing owner kind fails closed",
			input: Request{
				Owner: Owner{
					Name: "deepseek-r1",
					UID:  types.UID("550e8400-e29b-41d4-a716-446655440000"),
				},
				Identity: validIdentity(),
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{
						URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing owner name fails closed",
			input: Request{
				Owner: Owner{
					Kind: modelsv1alpha1.ModelKind,
					UID:  types.UID("550e8400-e29b-41d4-a716-446655440000"),
				},
				Identity: validIdentity(),
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{
						URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing owner uid fails closed",
			input: Request{
				Owner: Owner{
					Kind: modelsv1alpha1.ModelKind,
					Name: "deepseek-r1",
				},
				Identity: validIdentity(),
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{
						URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid identity fails closed",
			input: Request{
				Owner:    validOwner(),
				Identity: publication.Identity{},
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{
						URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing source fails closed",
			input: Request{
				Owner:    validOwner(),
				Identity: validIdentity(),
			},
			wantErr: true,
		},
		{
			name: "unsupported source scheme fails closed",
			input: Request{
				Owner:    validOwner(),
				Identity: validIdentity(),
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{
						URL: "oci://example.invalid/model",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "upload without payload fails closed",
			input: Request{
				Owner:    validOwner(),
				Identity: validIdentity(),
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{
						Upload: nil,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "upload source is valid only with upload payload",
			input: Request{
				Owner:    validOwner(),
				Identity: validIdentity(),
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{
						Upload: &modelsv1alpha1.UploadModelSource{},
					},
				},
			},
		},
		{
			name: "remote without URL fails closed",
			input: Request{
				Owner:    validOwner(),
				Identity: validIdentity(),
				Spec: modelsv1alpha1.ModelSpec{
					Source: modelsv1alpha1.ModelSourceSpec{},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.input.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func validOwner() Owner {
	return Owner{
		Kind:      modelsv1alpha1.ModelKind,
		Name:      "deepseek-r1",
		Namespace: "team-a",
		UID:       types.UID("550e8400-e29b-41d4-a716-446655440000"),
	}
}

func validIdentity() publication.Identity {
	return publication.Identity{
		Scope:     publication.ScopeNamespaced,
		Namespace: "team-a",
		Name:      "deepseek-r1",
	}
}
