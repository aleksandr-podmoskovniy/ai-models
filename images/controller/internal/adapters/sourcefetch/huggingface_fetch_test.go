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

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestFetchRemoteModelHuggingFaceUsesSnapshotDownloader(t *testing.T) {
	previousInfoFetcher := fetchHuggingFaceInfoFunc
	previousDownloaderFactory := newHuggingFaceSnapshotDownloader
	t.Cleanup(func() {
		fetchHuggingFaceInfoFunc = previousInfoFetcher
		newHuggingFaceSnapshotDownloader = previousDownloaderFactory
	})

	fetchHuggingFaceInfoFunc = func(context.Context, string, string, string) (HuggingFaceInfo, error) {
		return HuggingFaceInfo{
			ID:          "deepseek-ai/DeepSeek-R1",
			SHA:         "deadbeef",
			PipelineTag: "text-generation",
			License:     "mit",
			Files:       []string{"config.json", "model.safetensors"},
		}, nil
	}

	staging := newFakeUploadStaging()
	downloader := &fakeHuggingFaceSnapshotDownloader{}
	newHuggingFaceSnapshotDownloader = func() huggingFaceSnapshotDownloader {
		return downloader
	}

	result, err := FetchRemoteModel(t.Context(), RemoteOptions{
		URL:       "https://huggingface.co/deepseek-ai/DeepSeek-R1?revision=main",
		Workspace: t.TempDir(),
		HFToken:   "hf-token",
		RawStage: &RawStageOptions{
			Bucket:    "artifacts",
			KeyPrefix: "raw/1111-2222/source-url",
			Client:    staging,
		},
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}

	if got, want := downloader.input.Revision, "deadbeef"; got != want {
		t.Fatalf("unexpected downloader revision %q", got)
	}
	if got, want := result.InputFormat, modelsv1alpha1.ModelInputFormatSafetensors; got != want {
		t.Fatalf("unexpected input format %q", got)
	}
	if got, want := result.Provenance.ResolvedRevision, "deadbeef"; got != want {
		t.Fatalf("unexpected resolved revision %q", got)
	}
	if got, want := result.Provenance.ExternalReference, "deepseek-ai/DeepSeek-R1"; got != want {
		t.Fatalf("unexpected external reference %q", got)
	}
	if got, want := result.ProfileHints.TaskHint, "text-generation"; got != want {
		t.Fatalf("unexpected task hint %q", got)
	}
	if got, want := result.ProfileHints.License, "mit"; got != want {
		t.Fatalf("unexpected license %q", got)
	}
	if got, want := result.ProfileHints.SourceRepoID, "deepseek-ai/DeepSeek-R1"; got != want {
		t.Fatalf("unexpected source repo ID %q", got)
	}
	if got, want := len(result.StagedObjects), 2; got != want {
		t.Fatalf("unexpected staged object count %d", got)
	}
	if got, want := string(staging.objects["artifacts/raw/1111-2222/source-url/config.json"]), `{"architectures":["LlamaForCausalLM"]}`; got != want {
		t.Fatalf("unexpected raw config payload %q", got)
	}
	if got, want := string(staging.objects["artifacts/raw/1111-2222/source-url/model.safetensors"]), "tensor-payload"; got != want {
		t.Fatalf("unexpected raw model payload %q", got)
	}
	if payload, err := os.ReadFile(filepath.Join(result.ModelDir, "config.json")); err != nil {
		t.Fatalf("ReadFile(config.json) error = %v", err)
	} else if got, want := string(payload), `{"architectures":["LlamaForCausalLM"]}`; got != want {
		t.Fatalf("unexpected checkpoint config payload %q", got)
	}
	if payload, err := os.ReadFile(filepath.Join(result.ModelDir, "model.safetensors")); err != nil {
		t.Fatalf("ReadFile(model.safetensors) error = %v", err)
	} else if got, want := string(payload), "tensor-payload"; got != want {
		t.Fatalf("unexpected checkpoint model payload %q", got)
	}
}

type fakeHuggingFaceSnapshotDownloader struct {
	input huggingFaceSnapshotDownloadInput
}

func (f *fakeHuggingFaceSnapshotDownloader) Download(_ context.Context, input huggingFaceSnapshotDownloadInput) error {
	f.input = input
	if err := os.MkdirAll(input.SnapshotDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(input.SnapshotDir, "config.json"), []byte(`{"architectures":["LlamaForCausalLM"]}`), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(input.SnapshotDir, "model.safetensors"), []byte("tensor-payload"), 0o644); err != nil {
		return err
	}
	return nil
}
