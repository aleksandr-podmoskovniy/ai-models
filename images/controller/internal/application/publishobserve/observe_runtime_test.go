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

package publishobserve

import (
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestObserveSourceWorker(t *testing.T) {
	t.Parallel()

	request := testRequest()
	cases := []struct {
		name    string
		handle  *publicationports.SourceWorkerHandle
		assert  func(t *testing.T, got RuntimeObservationDecision)
		wantErr bool
	}{
		{
			name:   "missing worker fails closed without delete",
			handle: nil,
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseFailed {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Message != "source worker pod is missing" {
					t.Fatalf("unexpected message %q", got.Observation.Message)
				}
				if got.DeleteRuntime {
					t.Fatal("did not expect delete without runtime handle")
				}
			},
		},
		{
			name:   "running worker projects running observation",
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodRunning, "", nil),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseRunning {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.DeleteRuntime {
					t.Fatal("did not expect delete for running worker")
				}
			},
		},
		{
			name:   "succeeded worker decodes publication result",
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodSucceeded, succeededTerminationMessage(t), nil),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseSucceeded {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Snapshot == nil || got.Observation.Snapshot.Identity.Name != "deepseek-r1" {
					t.Fatalf("unexpected snapshot %#v", got.Observation.Snapshot)
				}
				if got.Observation.CleanupHandle == nil || got.Observation.CleanupHandle.Kind != cleanuphandle.KindBackendArtifact {
					t.Fatalf("unexpected cleanup handle %#v", got.Observation.CleanupHandle)
				}
				if !got.DeleteRuntime {
					t.Fatal("expected runtime delete after success")
				}
			},
		},
		{
			name:   "succeeded worker without result fails closed",
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodSucceeded, "   ", nil),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseFailed {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Message != "source worker pod completed without a result" {
					t.Fatalf("unexpected failure message %q", got.Observation.Message)
				}
				if !got.DeleteRuntime {
					t.Fatal("expected delete for malformed terminal result")
				}
			},
		},
		{
			name:   "succeeded worker with malformed result fails closed",
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodSucceeded, "not-json", nil),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseFailed {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Message == "" {
					t.Fatal("expected decode failure message")
				}
				if !got.DeleteRuntime {
					t.Fatal("expected delete for malformed result")
				}
			},
		},
		{
			name:   "failed worker keeps terminal message",
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodFailed, "hf import failed", nil),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseFailed {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Message != "hf import failed" {
					t.Fatalf("unexpected failure message %q", got.Observation.Message)
				}
				if !got.DeleteRuntime {
					t.Fatal("expected delete for failed worker")
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ObserveSourceWorker(request, tc.handle)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ObserveSourceWorker() error = %v", err)
			}
			tc.assert(t, got)
		})
	}
}

func TestObserveUploadSession(t *testing.T) {
	t.Parallel()

	request := testRequest()
	expiresAt := metav1.NewTime(time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC))
	cases := []struct {
		name   string
		handle *publicationports.UploadSessionHandle
		now    time.Time
		assert func(t *testing.T, got RuntimeObservationDecision)
	}{
		{
			name:   "missing session fails closed without delete",
			handle: nil,
			now:    time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseFailed {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Message != "upload session worker pod is missing" {
					t.Fatalf("unexpected failure message %q", got.Observation.Message)
				}
				if got.DeleteRuntime {
					t.Fatal("did not expect delete without runtime handle")
				}
			},
		},
		{
			name: "running session projects upload wait status",
			handle: publicationports.NewUploadSessionHandle("upload-a", corev1.PodRunning, "", modelsv1alpha1.ModelUploadStatus{
				ExternalURL:  "https://ai-models.example.com/upload/token",
				InClusterURL: "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:   "registry.example/upload",
				ExpiresAt:    &expiresAt,
			}, nil),
			now: time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseRunning {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Upload == nil || got.Observation.Upload.InClusterURL == "" {
					t.Fatalf("unexpected upload observation %#v", got.Observation.Upload)
				}
				if got.DeleteRuntime {
					t.Fatal("did not expect delete for active upload session")
				}
			},
		},
		{
			name: "expired session fails closed and deletes runtime",
			handle: publicationports.NewUploadSessionHandle("upload-a", corev1.PodRunning, "", modelsv1alpha1.ModelUploadStatus{
				ExternalURL:  "https://ai-models.example.com/upload/token",
				InClusterURL: "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:   "registry.example/upload",
				ExpiresAt:    &expiresAt,
			}, nil),
			now: time.Date(2026, 4, 7, 13, 0, 0, 0, time.UTC),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseFailed {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Message != "upload session expired before publication completed" {
					t.Fatalf("unexpected failure message %q", got.Observation.Message)
				}
				if !got.DeleteRuntime {
					t.Fatal("expected runtime delete for expired session")
				}
			},
		},
		{
			name: "failed session uses default failure message",
			handle: publicationports.NewUploadSessionHandle("upload-a", corev1.PodFailed, "   ", modelsv1alpha1.ModelUploadStatus{
				ExternalURL:  "https://ai-models.example.com/upload/token",
				InClusterURL: "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:   "registry.example/upload",
				ExpiresAt:    &expiresAt,
			}, nil),
			now: time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseFailed {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Message != "upload session worker pod failed" {
					t.Fatalf("unexpected failure message %q", got.Observation.Message)
				}
				if !got.DeleteRuntime {
					t.Fatal("expected delete for failed session")
				}
			},
		},
		{
			name: "completed session without result fails closed",
			handle: publicationports.NewUploadSessionHandle("upload-a", corev1.PodSucceeded, " ", modelsv1alpha1.ModelUploadStatus{
				ExternalURL:  "https://ai-models.example.com/upload/token",
				InClusterURL: "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:   "registry.example/upload",
				ExpiresAt:    &expiresAt,
			}, nil),
			now: time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseFailed {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.Message != "upload session completed without a staging result" {
					t.Fatalf("unexpected failure message %q", got.Observation.Message)
				}
				if !got.DeleteRuntime {
					t.Fatal("expected delete for empty terminal result")
				}
			},
		},
		{
			name: "completed session decodes staging handle",
			handle: publicationports.NewUploadSessionHandle("upload-a", corev1.PodSucceeded, uploadStagingTerminationMessage(t), modelsv1alpha1.ModelUploadStatus{
				ExternalURL:  "https://ai-models.example.com/upload/token",
				InClusterURL: "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:   "registry.example/upload",
				ExpiresAt:    &expiresAt,
			}, nil),
			now: time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseStaged {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.CleanupHandle == nil || got.Observation.CleanupHandle.Kind != cleanuphandle.KindUploadStaging {
					t.Fatalf("unexpected cleanup handle %#v", got.Observation.CleanupHandle)
				}
				if !got.DeleteRuntime {
					t.Fatal("expected runtime delete after success")
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ObserveUploadSession(request, tc.handle, tc.now)
			if err != nil {
				t.Fatalf("ObserveUploadSession() error = %v", err)
			}
			tc.assert(t, got)
		})
	}
}

func testRequest() publicationports.Request {
	return publicationports.Request{
		Owner: publicationports.Owner{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      "deepseek-r1",
			Namespace: "team-a",
			UID:       types.UID("550e8400-e29b-41d4-a716-446655440000"),
		},
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: "team-a",
			Name:      "deepseek-r1",
		},
		Spec: modelsv1alpha1.ModelSpec{
			Source: modelsv1alpha1.ModelSourceSpec{
				URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
			},
		},
	}
}

func succeededTerminationMessage(t *testing.T) string {
	t.Helper()

	payload, err := publicationartifact.EncodeResult(publicationartifact.Result{
		Artifact: publication.PublishedArtifact{
			Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:       "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			Digest:    "sha256:deadbeef",
			MediaType: "application/vnd.cncf.model.manifest.v1+json",
			SizeBytes: 42,
		},
		Resolved: publication.ResolvedProfile{
			Task:         "text-generation",
			Framework:    "transformers",
			Family:       "deepseek",
			License:      "apache-2.0",
			Architecture: "DeepseekForCausalLM",
			Format:       "Safetensors",
			SourceRepoID: "deepseek-ai/DeepSeek-R1",
		},
		Source: publication.SourceProvenance{
			Type:              modelsv1alpha1.ModelSourceTypeHuggingFace,
			ExternalReference: "deepseek-ai/DeepSeek-R1",
			ResolvedRevision:  "abc123",
		},
		CleanupHandle: cleanuphandle.Handle{
			Kind: cleanuphandle.KindBackendArtifact,
			Artifact: &cleanuphandle.ArtifactSnapshot{
				Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:  "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			},
			Backend: &cleanuphandle.BackendArtifactHandle{
				Reference: "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			},
		},
	})
	if err != nil {
		t.Fatalf("EncodeResult() error = %v", err)
	}
	return payload
}

func uploadStagingTerminationMessage(t *testing.T) string {
	t.Helper()

	payload, err := cleanuphandle.Encode(cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "ai-models",
			Key:       "raw/550e8400-e29b-41d4-a716-446655440000/model.gguf",
			FileName:  "model.gguf",
			SizeBytes: 42,
		},
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	return payload
}
