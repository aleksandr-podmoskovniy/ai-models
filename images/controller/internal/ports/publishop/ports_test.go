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

package publishop

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestSourceWorkerHandleHelpers(t *testing.T) {
	t.Parallel()

	deleted := false
	handle := NewSourceWorkerHandle("worker", corev1.PodSucceeded, `{"artifact":{"kind":"OCI","uri":"registry.example/model@sha256:deadbeef","digest":"sha256:deadbeef","mediaType":"application/vnd.cncf.model.manifest.v1+json"},"resolved":{"task":"text-generation"},"source":{"type":"HuggingFace","externalReference":"https://huggingface.co/test/model"},"cleanupHandle":{"kind":"BackendArtifact","artifact":{"kind":"OCI","uri":"registry.example/model@sha256:deadbeef"},"backend":{"reference":"registry.example/model@sha256:deadbeef"}}}`, "", "", func(context.Context) error {
		deleted = true
		return nil
	})

	if !handle.IsComplete() {
		t.Fatal("expected complete source worker handle")
	}
	if handle.IsFailed() {
		t.Fatal("did not expect failed source worker handle")
	}
	if err := handle.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("expected delete callback to be invoked")
	}
}

func TestUploadSessionHandleHelpers(t *testing.T) {
	t.Parallel()

	deleted := false
	handle := NewUploadSessionHandle("upload-worker", corev1.PodFailed, "upload failed", "37%", modelsv1alpha1.ModelUploadStatus{
		ExternalURL:              "https://ai-models.example.com/upload/token",
		InClusterURL:             "http://upload-a.d8-ai-models.svc:8444/upload/token",
		Repository:               "registry.example/upload",
		AuthorizationHeaderValue: "Bearer token-a",
	}, func(context.Context) error {
		deleted = true
		return nil
	})

	if handle.IsComplete() {
		t.Fatal("did not expect complete upload session handle")
	}
	if !handle.IsFailed() {
		t.Fatal("expected failed upload session handle")
	}
	if got, want := handle.Progress, "37%"; got != want {
		t.Fatalf("unexpected progress %q", got)
	}
	if err := handle.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("expected delete callback to be invoked")
	}
}
