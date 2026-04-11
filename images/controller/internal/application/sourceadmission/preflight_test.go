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
	"context"
	"errors"
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

	t.Run("http probe validates file name", func(t *testing.T) {
		t.Parallel()

		prober := &fakeHTTPProber{
			result: HTTPProbeResult{
				FileName:    "model.gguf",
				ContentType: "application/octet-stream",
			},
		}
		err := Preflight(t.Context(), PreflightInput{
			Owner:           owner,
			Identity:        identity,
			HTTPSourceProbe: prober,
			Spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://models.example.com/model.gguf",
				},
			},
		})
		if err != nil {
			t.Fatalf("Preflight() error = %v", err)
		}
		if prober.calls != 1 {
			t.Fatalf("expected one probe call, got %d", prober.calls)
		}
	})

	t.Run("http html pages are rejected", func(t *testing.T) {
		t.Parallel()

		err := Preflight(t.Context(), PreflightInput{
			Owner:    owner,
			Identity: identity,
			HTTPSourceProbe: &fakeHTTPProber{
				result: HTTPProbeResult{
					FileName:    "download",
					ContentType: "text/html; charset=utf-8",
				},
			},
			Spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://models.example.com/download",
				},
			},
		})
		if err == nil {
			t.Fatal("expected html source to be rejected")
		}
	})

	t.Run("huggingface skips http probe", func(t *testing.T) {
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

	t.Run("probe errors fail closed", func(t *testing.T) {
		t.Parallel()

		err := Preflight(t.Context(), PreflightInput{
			Owner:    owner,
			Identity: identity,
			HTTPSourceProbe: &fakeHTTPProber{
				err: errors.New("head failed"),
			},
			Spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://models.example.com/model.gguf",
				},
			},
		})
		if err == nil {
			t.Fatal("expected probe failure")
		}
	})

	t.Run("http sources require a probe client", func(t *testing.T) {
		t.Parallel()

		err := Preflight(t.Context(), PreflightInput{
			Owner:    owner,
			Identity: identity,
			Spec: modelsv1alpha1.ModelSpec{
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://models.example.com/model.gguf",
				},
			},
		})
		if err == nil {
			t.Fatal("expected missing probe client error")
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
			Owner:           owner,
			Identity:        identity,
			HTTPSourceProbe: &fakeHTTPProber{},
			Spec: modelsv1alpha1.ModelSpec{
				InputFormat: modelsv1alpha1.ModelInputFormat("Broken"),
				Source: modelsv1alpha1.ModelSourceSpec{
					URL: "https://models.example.com/model.gguf",
				},
			},
		})
		if err == nil {
			t.Fatal("expected invalid input format error")
		}
	})
}

type fakeHTTPProber struct {
	result HTTPProbeResult
	err    error
	calls  int
}

func (f *fakeHTTPProber) Probe(_ context.Context, _ HTTPProbeRequest) (HTTPProbeResult, error) {
	f.calls++
	if f.err != nil {
		return HTTPProbeResult{}, f.err
	}
	return f.result, nil
}
