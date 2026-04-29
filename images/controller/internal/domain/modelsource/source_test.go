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

package modelsource

import (
	"errors"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestDetectRemoteType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
		want modelsv1alpha1.ModelSourceType
		err  string
	}{
		{name: "huggingface root", url: "https://huggingface.co/deepseek-ai/DeepSeek-R1", want: modelsv1alpha1.ModelSourceTypeHuggingFace},
		{name: "huggingface tree", url: "https://huggingface.co/deepseek-ai/DeepSeek-R1/tree/main", want: modelsv1alpha1.ModelSourceTypeHuggingFace},
		{name: "ollama library", url: "https://ollama.com/library/qwen3.6", want: modelsv1alpha1.ModelSourceTypeOllama},
		{name: "ollama library tag", url: "https://ollama.com/library/qwen3.6:latest", want: modelsv1alpha1.ModelSourceTypeOllama},
		{name: "plain http rejected", url: "http://huggingface.co/deepseek-ai/DeepSeek-R1", err: `unsupported source URL scheme "http"`},
		{name: "generic http rejected", url: "https://downloads.example.com/model.tar.gz", err: `unsupported source URL host "downloads.example.com"`},
		{name: "ollama non-library rejected", url: "https://ollama.com/search?q=qwen", err: `unsupported source URL path "/search"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DetectRemoteType(tc.url)
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("DetectRemoteType() error = %v, want %q", err, tc.err)
				}
				if !IsUnsupportedRemoteError(err) {
					t.Fatalf("expected unsupported remote source classification, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectRemoteType() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("DetectRemoteType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDetectType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source modelsv1alpha1.ModelSourceSpec
		want   modelsv1alpha1.ModelSourceType
		err    string
	}{
		{
			name: "upload",
			source: modelsv1alpha1.ModelSourceSpec{
				Upload: &modelsv1alpha1.UploadModelSource{},
			},
			want: modelsv1alpha1.ModelSourceTypeUpload,
		},
		{
			name: "remote",
			source: modelsv1alpha1.ModelSourceSpec{
				URL: "https://hf.co/deepseek-ai/DeepSeek-R1",
			},
			want: modelsv1alpha1.ModelSourceTypeHuggingFace,
		},
		{
			name: "both url and upload rejected",
			source: modelsv1alpha1.ModelSourceSpec{
				URL:    "https://hf.co/deepseek-ai/DeepSeek-R1",
				Upload: &modelsv1alpha1.UploadModelSource{},
			},
			err: "exactly one of source.url or source.upload must be specified",
		},
		{
			name:   "empty rejected",
			source: modelsv1alpha1.ModelSourceSpec{},
			err:    "source.url or source.upload must be specified",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DetectType(tc.source)
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("DetectType() error = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectType() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("DetectType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsUnsupportedRemoteError(t *testing.T) {
	t.Parallel()

	if IsUnsupportedRemoteError(nil) {
		t.Fatal("nil error must not be classified as unsupported source")
	}
	if IsUnsupportedRemoteError(errors.New("something else")) {
		t.Fatal("unrelated error must not be classified as unsupported source")
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

func TestParseHuggingFaceURLUsesRevisionQuery(t *testing.T) {
	t.Parallel()

	repoID, revision, err := ParseHuggingFaceURL("https://huggingface.co/deepseek-ai/DeepSeek-R1?revision=refs/pr/1")
	if err != nil {
		t.Fatalf("ParseHuggingFaceURL() error = %v", err)
	}
	if repoID != "deepseek-ai/DeepSeek-R1" || revision != "refs/pr/1" {
		t.Fatalf("unexpected repo/revision %q %q", repoID, revision)
	}
}

func TestParseHuggingFaceURLRejectsMissingRepo(t *testing.T) {
	t.Parallel()

	_, _, err := ParseHuggingFaceURL("https://huggingface.co/deepseek-ai")
	if err == nil || err.Error() != "huggingface URL must contain owner/repo" {
		t.Fatalf("ParseHuggingFaceURL() error = %v, want missing repo", err)
	}
}

func TestParseOllamaLibraryURL(t *testing.T) {
	t.Parallel()

	name, tag, err := ParseOllamaLibraryURL("https://ollama.com/library/qwen3.6:latest")
	if err != nil {
		t.Fatalf("ParseOllamaLibraryURL() error = %v", err)
	}
	if name != "qwen3.6" || tag != "latest" {
		t.Fatalf("unexpected name/tag %q %q", name, tag)
	}
}

func TestParseOllamaLibraryURLAllowsImplicitTag(t *testing.T) {
	t.Parallel()

	name, tag, err := ParseOllamaLibraryURL("https://ollama.com/library/qwen3.6")
	if err != nil {
		t.Fatalf("ParseOllamaLibraryURL() error = %v", err)
	}
	if name != "qwen3.6" || tag != "" {
		t.Fatalf("unexpected name/tag %q %q", name, tag)
	}
}

func TestDetectTypeRejectsUnsupportedProvider(t *testing.T) {
	t.Parallel()

	_, err := DetectType(modelsv1alpha1.ModelSourceSpec{
		URL: "https://downloads.example.com/model.tar.gz",
		AuthSecretRef: &modelsv1alpha1.SecretReference{
			Name: "remote-auth",
		},
	})
	if err == nil {
		t.Fatal("expected unsupported source error")
	}
	if !IsUnsupportedRemoteError(err) {
		t.Fatalf("expected unsupported remote source classification, got %v", err)
	}
}
