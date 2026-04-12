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

package sourceadmission

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestPreflight(t *testing.T) {
	t.Parallel()

	owner := ingestadmission.OwnerBinding{
		Kind:      modelsv1alpha1.ModelKind,
		Name:      "deepseek-r1",
		Namespace: "team-a",
		UID:       "1111-2222",
	}
	identity := publicationdata.Identity{
		Scope:     publicationdata.ScopeNamespaced,
		Namespace: "team-a",
		Name:      "deepseek-r1",
	}

	t.Run("huggingface passes", func(t *testing.T) {
		t.Parallel()

		err := Preflight(t.Context(), PreflightInput{
			Owner:    owner,
			Identity: identity,
			Spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
				},
			},
		})
		if err != nil {
			t.Fatalf("Preflight() error = %v", err)
		}
	})

	t.Run("upload sources skip remote probing", func(t *testing.T) {
		t.Parallel()

		err := Preflight(t.Context(), PreflightInput{
			Owner:    owner,
			Identity: identity,
			Spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					Upload: &modelsv1alpha1.UploadModelSource{},
				},
			},
		})
		if err != nil {
			t.Fatalf("Preflight() error = %v", err)
		}
	})

	t.Run("invalid declared format fails closed", func(t *testing.T) {
		t.Parallel()

		err := Preflight(t.Context(), PreflightInput{
			Owner:    owner,
			Identity: identity,
			Spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormat("Broken"),
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
				},
			},
		})
		if err == nil {
			t.Fatal("expected invalid input format error")
		}
	})

	t.Run("invalid owner binding fails closed", func(t *testing.T) {
		t.Parallel()

		err := Preflight(t.Context(), PreflightInput{
			Owner: ingestadmission.OwnerBinding{
				Kind: modelsv1alpha1.ModelKind,
				Name: "other-model",
				UID:  owner.UID,
			},
			Identity: identity,
			Spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
				},
			},
		})
		if err == nil {
			t.Fatal("expected owner binding error")
		}
	})

	t.Run("missing source fails closed", func(t *testing.T) {
		t.Parallel()

		err := Preflight(t.Context(), PreflightInput{
			Owner:    owner,
			Identity: identity,
			Spec:     modelsv1alpha1.ModelSpec{},
		})
		if err == nil {
			t.Fatal("expected source validation error")
		}
	})
}
