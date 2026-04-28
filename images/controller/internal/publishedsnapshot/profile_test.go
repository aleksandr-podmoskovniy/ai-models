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

package publishedsnapshot

import "testing"

func TestResolvedProfileHasPartialConfidence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		profile ResolvedProfile
		want    bool
	}{
		{
			name: "empty low confidence marker ignored",
			profile: ResolvedProfile{
				TaskConfidence: ProfileConfidenceHint,
			},
			want: false,
		},
		{
			name: "reliable public facts are not partial",
			profile: ResolvedProfile{
				Task:                          "text-generation",
				TaskConfidence:                ProfileConfidenceDeclared,
				Architecture:                  "LlamaForCausalLM",
				ArchitectureConfidence:        ProfileConfidenceExact,
				ContextWindowTokens:           8192,
				ContextWindowTokensConfidence: ProfileConfidenceExact,
			},
			want: false,
		},
		{
			name: "hint architecture is partial",
			profile: ResolvedProfile{
				Architecture:           "guessed",
				ArchitectureConfidence: ProfileConfidenceHint,
			},
			want: true,
		},
		{
			name: "estimated context window is partial",
			profile: ResolvedProfile{
				ContextWindowTokens:           8192,
				ContextWindowTokensConfidence: ProfileConfidenceEstimated,
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.profile.HasPartialConfidence(); got != tc.want {
				t.Fatalf("HasPartialConfidence() = %v, want %v", got, tc.want)
			}
		})
	}
}
