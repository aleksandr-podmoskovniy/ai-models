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

package garbagecollection

import "testing"

func TestInferRepositoryMetadataPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		want  string
		found bool
	}{
		{
			name:  "namespaced model repository",
			key:   "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/gemma/1111/_layers/sha256/abc/link",
			want:  "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/gemma/1111",
			found: true,
		},
		{
			name:  "cluster model repository",
			key:   "dmcr/docker/registry/v2/repositories/ai-models/catalog/cluster/gemma/2222/_manifests/revisions/sha256/deadbeef/link",
			want:  "dmcr/docker/registry/v2/repositories/ai-models/catalog/cluster/gemma/2222",
			found: true,
		},
		{
			name:  "non model repository tree",
			key:   "dmcr/docker/registry/v2/repositories/library/busybox/_layers/sha256/abc/link",
			found: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, found := inferRepositoryMetadataPrefix("dmcr", test.key)
			if found != test.found {
				t.Fatalf("found = %v, want %v", found, test.found)
			}
			if got != test.want {
				t.Fatalf("prefix = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInferSourceMirrorPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		want  string
		found bool
	}{
		{
			name:  "mirror file object",
			key:   "raw/1111/source-url/.mirror/huggingface/google/gemma/deadbeef/files/model.safetensors",
			want:  "raw/1111/source-url/.mirror/huggingface/google/gemma/deadbeef",
			found: true,
		},
		{
			name:  "mirror manifest object",
			key:   "raw/1111/source-url/.mirror/huggingface/google/gemma/deadbeef/manifest.json",
			want:  "raw/1111/source-url/.mirror/huggingface/google/gemma/deadbeef",
			found: true,
		},
		{
			name:  "plain upload staging key",
			key:   "raw/1111/model.gguf",
			found: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, found := inferSourceMirrorPrefix(test.key)
			if found != test.found {
				t.Fatalf("found = %v, want %v", found, test.found)
			}
			if got != test.want {
				t.Fatalf("prefix = %q, want %q", got, test.want)
			}
		})
	}
}
