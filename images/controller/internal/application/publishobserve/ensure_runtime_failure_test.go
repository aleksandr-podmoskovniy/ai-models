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

	publicationplan "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
)

func TestEnsureRuntimeObservationReturnsRuntimeErrors(t *testing.T) {
	t.Parallel()

	owner := testkit.NewModel()

	t.Run("source worker runtime error is returned", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("create worker pod")
		_, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context:       context.Background(),
			Owner:         owner,
			Request:       testRequest(),
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
}

func TestEnsureRuntimeObservationFailsClosed(t *testing.T) {
	t.Parallel()

	owner := testkit.NewModel()

	t.Run("missing source worker runtime", func(t *testing.T) {
		t.Parallel()

		_, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context: context.Background(),
			Owner:   owner,
			Request: testRequest(),
			Mode:    publicationplan.ExecutionModeSourceWorker,
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing upload session runtime", func(t *testing.T) {
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

	t.Run("unsupported execution mode", func(t *testing.T) {
		t.Parallel()

		_, err := EnsureRuntimeObservation(EnsureRuntimeObservationInput{
			Context: context.Background(),
			Owner:   owner,
			Request: testRequest(),
			Mode:    publicationplan.ExecutionMode("Unknown"),
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
