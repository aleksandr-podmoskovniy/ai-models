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

package v1alpha1

import "testing"

func TestDetectRemoteSourceType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
		want ModelSourceType
	}{
		{name: "huggingface root", url: "https://huggingface.co/deepseek-ai/DeepSeek-R1", want: ModelSourceTypeHuggingFace},
		{name: "huggingface tree", url: "https://huggingface.co/deepseek-ai/DeepSeek-R1/tree/main", want: ModelSourceTypeHuggingFace},
		{name: "generic http", url: "https://downloads.example.com/model.tar.gz", want: ModelSourceTypeHTTP},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DetectRemoteSourceType(tc.url)
			if err != nil {
				t.Fatalf("DetectRemoteSourceType() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("DetectRemoteSourceType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseHuggingFaceURL(t *testing.T) {
	t.Parallel()

	repoID, revision, err := ParseHuggingFaceURL("https://huggingface.co/deepseek-ai/DeepSeek-R1/tree/main")
	if err != nil {
		t.Fatalf("ParseHuggingFaceURL() error = %v", err)
	}
	if repoID != "deepseek-ai/DeepSeek-R1" || revision != "main" {
		t.Fatalf("unexpected repo/revision %q %q", repoID, revision)
	}
}
