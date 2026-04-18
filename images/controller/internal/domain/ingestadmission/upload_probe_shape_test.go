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

package ingestadmission

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestValidateUploadProbeShape(t *testing.T) {
	t.Parallel()

	t.Run("accepts direct gguf without owner context", func(t *testing.T) {
		t.Parallel()

		got, err := ValidateUploadProbeShape("", UploadProbeInput{
			FileName: "model.gguf",
			Chunk:    []byte("GGUFpayload"),
		})
		if err != nil {
			t.Fatalf("ValidateUploadProbeShape() error = %v", err)
		}
		if got.ResolvedInputFormat != modelsv1alpha1.ModelInputFormatGGUF {
			t.Fatalf("unexpected resolved format %q", got.ResolvedInputFormat)
		}
	})

	t.Run("rejects direct safetensors without owner context", func(t *testing.T) {
		t.Parallel()

		_, err := ValidateUploadProbeShape("", UploadProbeInput{
			FileName: "model.safetensors",
			Chunk:    []byte("header"),
		})
		if err == nil {
			t.Fatal("expected direct safetensors probe to fail")
		}
	})
}
