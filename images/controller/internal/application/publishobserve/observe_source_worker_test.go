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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
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
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodRunning, "", modelsv1alpha1.ModelConditionReasonPublicationUploading, "123/456 bytes uploaded", nil),
			assert: func(t *testing.T, got RuntimeObservationDecision) {
				t.Helper()
				if got.Observation.Phase != publicationdomain.OperationPhaseRunning {
					t.Fatalf("unexpected phase %q", got.Observation.Phase)
				}
				if got.Observation.ConditionReason != modelsv1alpha1.ModelConditionReasonPublicationUploading {
					t.Fatalf("unexpected running reason %q", got.Observation.ConditionReason)
				}
				if got.Observation.Message != "123/456 bytes uploaded" {
					t.Fatalf("unexpected running message %q", got.Observation.Message)
				}
				if got.DeleteRuntime {
					t.Fatal("did not expect delete for running worker")
				}
			},
		},
		{
			name:   "succeeded worker decodes publication result",
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodSucceeded, succeededTerminationMessage(t), "", "", nil),
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
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodSucceeded, "   ", "", "", nil),
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
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodSucceeded, "not-json", "", "", nil),
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
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodFailed, "hf import failed", "", "", nil),
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
