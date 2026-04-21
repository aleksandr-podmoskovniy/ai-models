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

package publishstate

import (
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestObserveSourceWorker(t *testing.T) {
	t.Parallel()

	success := &PublicationSuccess{
		Snapshot: publicationdata.Snapshot{
			Artifact: publicationdata.PublishedArtifact{
				Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:    "registry.example/model@sha256:deadbeef",
				Digest: "sha256:deadbeef",
			},
		},
		CleanupHandle: cleanuphandle.Handle{Kind: cleanuphandle.KindBackendArtifact},
	}

	testCases := []struct {
		name    string
		input   SourceWorkerObservation
		want    SourceWorkerDecision
		wantErr bool
	}{
		{
			name: "created persists running worker",
			input: SourceWorkerObservation{
				Current:    OperationStatusView{Phase: OperationPhasePending},
				WorkerName: "worker-a",
				Created:    true,
				State:      RuntimeStateRunning,
			},
			want: SourceWorkerDecision{
				PersistRunning: true,
				RunningWorker:  "worker-a",
			},
		},
		{
			name: "awaiting result requeues",
			input: SourceWorkerObservation{
				Current:      OperationStatusView{Phase: OperationPhaseRunning, WorkerName: "worker-a"},
				WorkerName:   "worker-a",
				State:        RuntimeStateAwaitingResult,
				RequeueAfter: 2 * time.Second,
			},
			want: SourceWorkerDecision{RequeueAfter: 2 * time.Second},
		},
		{
			name: "success deletes worker",
			input: SourceWorkerObservation{
				State:   RuntimeStateSucceeded,
				Success: success,
			},
			want: SourceWorkerDecision{
				Success:      success,
				DeleteWorker: true,
			},
		},
		{
			name: "failed deletes worker",
			input: SourceWorkerObservation{
				State:   RuntimeStateFailed,
				Failure: "boom",
			},
			want: SourceWorkerDecision{
				FailMessage:  "boom",
				DeleteWorker: true,
			},
		},
		{
			name: "success without payload fails closed",
			input: SourceWorkerObservation{
				State: RuntimeStateSucceeded,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ObserveSourceWorker(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ObserveSourceWorker() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected decision %#v", got)
			}
		})
	}
}

func TestObserveUploadSession(t *testing.T) {
	t.Parallel()

	expiresAt := metav1.NewTime(time.Unix(1712345678, 0).UTC())
	stagedHandle := &cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:   "ai-models",
			Key:      "raw/1111-2222/model.gguf",
			FileName: "model.gguf",
		},
	}
	uploadStatus := &modelsv1alpha1.ModelUploadStatus{
		ExternalURL:              "https://ai-models.example.com/upload/token",
		InClusterURL:             "http://upload-a.d8-ai-models.svc:8444/upload/token",
		Repository:               "registry.example/upload",
		AuthorizationHeaderValue: "Bearer token-a",
		ExpiresAt:                &expiresAt,
	}

	testCases := []struct {
		name    string
		input   UploadSessionObservation
		want    UploadSessionDecision
		wantErr bool
	}{
		{
			name: "created session persists running and upload",
			input: UploadSessionObservation{
				Current:      OperationStatusView{Phase: OperationPhasePending},
				WorkerName:   "upload-a",
				Created:      true,
				State:        RuntimeStateRunning,
				UploadStatus: uploadStatus,
			},
			want: UploadSessionDecision{
				PersistRunning: true,
				RunningWorker:  "upload-a",
				PersistUpload:  true,
				UploadStatus:   uploadStatus,
			},
		},
		{
			name: "expired session fails and deletes",
			input: UploadSessionObservation{
				State:   RuntimeStateRunning,
				Expired: true,
			},
			want: UploadSessionDecision{
				FailMessage:   "upload session expired before publication completed",
				DeleteSession: true,
			},
		},
		{
			name: "success deletes session",
			input: UploadSessionObservation{
				State:        RuntimeStateSucceeded,
				StagedHandle: stagedHandle,
			},
			want: UploadSessionDecision{
				StagedHandle:  stagedHandle,
				DeleteSession: true,
			},
		},
		{
			name: "failure deletes session",
			input: UploadSessionObservation{
				State:   RuntimeStateFailed,
				Failure: "boom",
			},
			want: UploadSessionDecision{
				FailMessage:   "boom",
				DeleteSession: true,
			},
		},
		{
			name: "unknown state fails closed",
			input: UploadSessionObservation{
				State: RuntimeState("Unknown"),
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ObserveUploadSession(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ObserveUploadSession() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected decision %#v", got)
			}
		})
	}
}
