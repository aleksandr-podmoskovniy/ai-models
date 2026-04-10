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
	"errors"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationplan "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEnsureRuntimeObservation(t *testing.T) {
	t.Parallel()

	owner := testkit.NewModel()
	request := testRequest()
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	t.Run("source worker mode routes through source worker runtime", func(t *testing.T) {
		t.Parallel()

		deleted := false
		sourceWorkers := &fakeSourceWorkerRuntime{
			handle: publicationports.NewSourceWorkerHandle("worker-a", corev1.PodRunning, "", func(context.Context) error {
				deleted = true
				return nil
			}),
		}
		uploadSessions := &fakeUploadSessionRuntime{}

		got, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context:        context.Background(),
			Owner:          owner,
			Request:        request,
			Mode:           publicationplan.ExecutionModeSourceWorker,
			SourceWorkers:  sourceWorkers,
			UploadSessions: uploadSessions,
			Now:            now,
		})
		if err != nil {
			t.Fatalf("EnsureRuntimeObservation() error = %v", err)
		}
		if sourceWorkers.calls != 1 || uploadSessions.calls != 0 {
			t.Fatalf("unexpected runtime calls source=%d upload=%d", sourceWorkers.calls, uploadSessions.calls)
		}
		if got.Decision.Observation.Phase != publicationdomain.OperationPhaseRunning {
			t.Fatalf("unexpected observation phase %q", got.Decision.Observation.Phase)
		}
		if got.DeleteFn == nil {
			t.Fatal("expected delete function for source worker handle")
		}
		if err := got.DeleteFn(context.Background()); err != nil {
			t.Fatalf("DeleteFn() error = %v", err)
		}
		if !deleted {
			t.Fatal("expected source worker delete callback to run")
		}
	})

	t.Run("upload mode routes through upload session runtime", func(t *testing.T) {
		t.Parallel()

		deleted := false
		sourceWorkers := &fakeSourceWorkerRuntime{}
		uploadSessions := &fakeUploadSessionRuntime{
			handle: publicationports.NewUploadSessionHandle("upload-a", corev1.PodRunning, "", modelsv1alpha1.ModelUploadStatus{
				ExternalURL:  "https://ai-models.example.com/upload/token",
				InClusterURL: "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:   "registry.example/upload",
			}, func(context.Context) error {
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
		if got.DeleteFn == nil {
			t.Fatal("expected delete function for upload session handle")
		}
		if err := got.DeleteFn(context.Background()); err != nil {
			t.Fatalf("DeleteFn() error = %v", err)
		}
		if !deleted {
			t.Fatal("expected upload session delete callback to run")
		}
	})

	t.Run("runtime error is returned", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("create worker pod")
		_, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context:       context.Background(),
			Owner:         owner,
			Request:       request,
			Mode:          publicationplan.ExecutionModeSourceWorker,
			SourceWorkers: &fakeSourceWorkerRuntime{err: wantErr},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("unexpected error %v", err)
		}
	})

	t.Run("upload runtime error is returned", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("create upload pod")
		_, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context:        context.Background(),
			Owner:          owner,
			Request:        uploadRequest(),
			Mode:           publicationplan.ExecutionModeUpload,
			UploadSessions: &fakeUploadSessionRuntime{err: wantErr},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("unexpected error %v", err)
		}
	})

	t.Run("missing source worker runtime fails closed", func(t *testing.T) {
		t.Parallel()

		_, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context: context.Background(),
			Owner:   owner,
			Request: request,
			Mode:    publicationplan.ExecutionModeSourceWorker,
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing upload session runtime fails closed", func(t *testing.T) {
		t.Parallel()

		_, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context: context.Background(),
			Owner:   owner,
			Request: uploadRequest(),
			Mode:    publicationplan.ExecutionModeUpload,
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unsupported execution mode fails closed", func(t *testing.T) {
		t.Parallel()

		_, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context: context.Background(),
			Owner:   owner,
			Request: request,
			Mode:    publicationplan.ExecutionMode("Unknown"),
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRuntimeObservationNowDefaultsToCurrentUTC(t *testing.T) {
	t.Parallel()

	if got := runtimeObservationNow(time.Time{}); got.IsZero() {
		t.Fatal("expected non-zero current time")
	}
	if got := runtimeObservationNow(time.Date(2026, 4, 7, 12, 0, 0, 0, time.FixedZone("custom", 3*60*60))); got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", got.Location())
	}
}

type fakeSourceWorkerRuntime struct {
	handle *publicationports.SourceWorkerHandle
	err    error
	calls  int
}

func (f *fakeSourceWorkerRuntime) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.OperationContext) (*publicationports.SourceWorkerHandle, bool, error) {
	f.calls++
	return f.handle, false, f.err
}

type fakeUploadSessionRuntime struct {
	handle *publicationports.UploadSessionHandle
	err    error
	calls  int
}

func (f *fakeUploadSessionRuntime) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.OperationContext) (*publicationports.UploadSessionHandle, bool, error) {
	f.calls++
	return f.handle, false, f.err
}

func uploadRequest() publicationports.Request {
	request := testRequest()
	request.Owner.Name = "deepseek-r1-upload"
	request.Identity.Name = "deepseek-r1-upload"
	request.Spec.Source = modelsv1alpha1.ModelSourceSpec{
		Upload: &modelsv1alpha1.UploadModelSource{},
	}
	request.Spec.RuntimeHints = &modelsv1alpha1.ModelRuntimeHints{
		Task: "text-generation",
	}
	return request
}
