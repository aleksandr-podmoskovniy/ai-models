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
	"context"
	"testing"
	"time"

	publicationplan "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
)

func TestEnsureRuntimeObservationUploadMode(t *testing.T) {
	t.Parallel()

	owner := testkit.NewModel()
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	deleted := false
	sourceWorkers := &fakeSourceWorkerRuntime{}
	uploadSessions := &fakeUploadSessionRuntime{
		handle: publicationports.NewUploadSessionHandle("upload-a", corev1.PodRunning, "", "23%", testUploadStatusForObservation(nil), func(context.Context) error {
			deleted = true
			return nil
		}),
	}

	got, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
		Context:        context.Background(),
		Owner:          owner,
		Request:        uploadRequest(),
		Mode:           publicationplan.ExecutionModeUpload,
		SourceWorkers:  sourceWorkers,
		UploadSessions: uploadSessions,
		Now:            now,
	})
	if err != nil {
		t.Fatalf("EnsureRuntimeObservation() error = %v", err)
	}
	if sourceWorkers.calls != 0 || uploadSessions.calls != 1 {
		t.Fatalf("unexpected runtime calls source=%d upload=%d", sourceWorkers.calls, uploadSessions.calls)
	}
	if got.Decision.Observation.Phase != publicationdomain.OperationPhaseRunning {
		t.Fatalf("unexpected observation phase %q", got.Decision.Observation.Phase)
	}
	if got.Decision.Observation.Upload == nil || got.Decision.Observation.Upload.InClusterURL == "" {
		t.Fatalf("unexpected upload observation %#v", got.Decision.Observation.Upload)
	}
	if got.Decision.Observation.Progress != "23%" {
		t.Fatalf("unexpected progress %q", got.Decision.Observation.Progress)
	}
	if got.DeleteFn == nil {
		t.Fatal("expected delete function for upload session handle")
	}
	if err := got.DeleteFn(context.Background()); err != nil {
		t.Fatalf("DeleteFn() error = %v", err)
	}
	if !deleted {
		t.Fatal("expected upload session delete callback to run")
	}
}
