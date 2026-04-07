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

package sourcefetch

import "testing"

func TestHuggingFaceInfoURL(t *testing.T) {
	t.Parallel()

	endpoint, err := huggingFaceInfoURL("deepseek-ai/DeepSeek-R1", "main")
	if err != nil {
		t.Fatalf("huggingFaceInfoURL() error = %v", err)
	}
	if got, want := endpoint, "https://huggingface.co/api/models/deepseek-ai/DeepSeek-R1?revision=main"; got != want {
		t.Fatalf("unexpected endpoint %q", got)
	}
}

func TestHuggingFaceResolveURL(t *testing.T) {
	t.Parallel()

	endpoint, err := huggingFaceResolveURL("deepseek-ai/DeepSeek-R1", "abc123", "config.json")
	if err != nil {
		t.Fatalf("huggingFaceResolveURL() error = %v", err)
	}
	if got, want := endpoint, "https://huggingface.co/deepseek-ai/DeepSeek-R1/resolve/abc123/config.json?download=1"; got != want {
		t.Fatalf("unexpected resolve endpoint %q", got)
	}
}

func TestResolvedHuggingFaceRevision(t *testing.T) {
	t.Parallel()

	if got, want := ResolveHuggingFaceRevision(HuggingFaceInfo{SHA: "deadbeef"}, "main"), "deadbeef"; got != want {
		t.Fatalf("unexpected revision %q", got)
	}
}
