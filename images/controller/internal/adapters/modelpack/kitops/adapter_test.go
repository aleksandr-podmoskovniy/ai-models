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

package kitops

import "testing"

func TestRegistryFromOCIReference(t *testing.T) {
	t.Parallel()

	if got, want := registryFromOCIReference("registry.example.com/ai-models/catalog/model:published"), "registry.example.com"; got != want {
		t.Fatalf("unexpected registry %q", got)
	}
}

func TestImmutableOCIReference(t *testing.T) {
	t.Parallel()

	if got, want := immutableOCIReference("registry.example.com/ai-models/catalog/model:published", "sha256:deadbeef"), "registry.example.com/ai-models/catalog/model@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected immutable reference %q", got)
	}
}

func TestInspectModelPackSize(t *testing.T) {
	t.Parallel()

	size := inspectModelPackSize(map[string]any{
		"manifest": map[string]any{
			"config": map[string]any{"size": float64(10)},
			"layers": []any{
				map[string]any{"size": float64(30)},
				map[string]any{"size": float64(40)},
			},
		},
	})

	if got, want := size, int64(80); got != want {
		t.Fatalf("unexpected size %d", got)
	}
}
