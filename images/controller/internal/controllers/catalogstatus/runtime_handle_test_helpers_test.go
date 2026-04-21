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

package catalogstatus

import (
	"context"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
			Task:                         "text-generation",
			Framework:                    "transformers",
			Family:                       "deepseek",
			License:                      "apache-2.0",
			Architecture:                 "DeepseekForCausalLM",
			Format:                       "Safetensors",
			ContextWindowTokens:          8192,
			SourceRepoID:                 "deepseek-ai/DeepSeek-R1",
			SupportedEndpointTypes:       []string{"Chat", "TextGeneration"},
			CompatibleAcceleratorVendors: []string{"NVIDIA"},
			CompatiblePrecisions:         []string{"BF16"},
		},
		Source: publication.SourceProvenance{
			Type:              modelsv1alpha1.ModelSourceTypeHuggingFace,
			ExternalReference: "deepseek-ai/DeepSeek-R1",
			ResolvedRevision:  "abc123",
			RawURI:            "s3://artifacts/raw/550e8400-e29b-41d4-a716-446655440000/source-url",
			RawObjectCount:    4,
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

func runningSourceWorkerHandle() *publicationports.SourceWorkerHandle {
	return publicationports.NewSourceWorkerHandle("publish-worker", corev1.PodRunning, "", "", "", nil)
}

func failedSourceWorkerHandle(message string) *publicationports.SourceWorkerHandle {
	return publicationports.NewSourceWorkerHandle("publish-worker", corev1.PodFailed, message, "", "", nil)
}

func succeededSourceWorkerHandle(t *testing.T, deleted *bool) *publicationports.SourceWorkerHandle {
	t.Helper()
	return publicationports.NewSourceWorkerHandle("publish-worker", corev1.PodSucceeded, succeededTerminationMessage(t), "", "", func(context.Context) error {
		if deleted != nil {
			*deleted = true
		}
		return nil
	})
}

func runningUploadSessionHandle() *publicationports.UploadSessionHandle {
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	return publicationports.NewUploadSessionHandle(
		"upload-worker",
		corev1.PodRunning,
		"",
		"17%",
		modelsv1alpha1.ModelUploadStatus{
			ExpiresAt:                &metav1.Time{Time: expiresAt},
			Repository:               "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1-upload/550e8400-e29b-41d4-a716-446655440111:published",
			ExternalURL:              "https://ai-models.example.com/upload/token",
			InClusterURL:             "http://upload-worker.d8-ai-models.svc:8444/upload/token",
			AuthorizationHeaderValue: "Bearer token-a",
		},
		nil,
	)
}

func succeededUploadSessionHandle(t *testing.T, deleted *bool) *publicationports.UploadSessionHandle {
	t.Helper()

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "ai-models",
			Key:       "raw/1111-2222/model.gguf",
			FileName:  "model.gguf",
			SizeBytes: 128,
		},
	}
	encoded, err := cleanuphandle.Encode(handle)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	return publicationports.NewUploadSessionHandle(
		"upload-worker",
		corev1.PodSucceeded,
		encoded,
		"",
		modelsv1alpha1.ModelUploadStatus{},
		func(context.Context) error {
			if deleted != nil {
				*deleted = true
			}
			return nil
		},
	)
}
