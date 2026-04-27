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

package backendprefix

import "testing"

func TestRepositoryMetadataPrefixFromReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		reference string
		want      string
	}{
		{
			name:      "digest",
			reference: "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/team-a/model/1111@sha256:deadbeef",
			want:      "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111",
		},
		{
			name:      "tag",
			reference: "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/team-a/model/1111:published",
			want:      "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/model/1111",
		},
		{
			name:      "registry-port",
			reference: "registry.example.com:5000/ai-models/catalog/model:published",
			want:      "dmcr/docker/registry/v2/repositories/ai-models/catalog/model",
		},
		{
			name:      "whitespace",
			reference: "  registry.example.com/ai-models/catalog/model@sha256:deadbeef  ",
			want:      "dmcr/docker/registry/v2/repositories/ai-models/catalog/model",
		},
		{
			name:      "missing-registry",
			reference: "ai-models/catalog/model:published",
			want:      "",
		},
		{
			name:      "scheme",
			reference: "https://registry.example.com/ai-models/catalog/model:published",
			want:      "",
		},
		{
			name:      "path-traversal",
			reference: "registry.example.com/ai-models/../model:published",
			want:      "",
		},
		{
			name:      "empty-path-segment",
			reference: "registry.example.com/ai-models//model:published",
			want:      "",
		},
		{
			name:      "backslash-path",
			reference: `registry.example.com/ai-models\model:published`,
			want:      "",
		},
		{
			name:      "empty",
			reference: " ",
			want:      "",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := RepositoryMetadataPrefixFromReference(test.reference); got != test.want {
				t.Fatalf("RepositoryMetadataPrefixFromReference() = %q, want %q", got, test.want)
			}
		})
	}
}
