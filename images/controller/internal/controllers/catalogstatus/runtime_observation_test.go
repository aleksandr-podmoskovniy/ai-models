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
	"errors"
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
)

func TestEnsureRuntimeObservationErrorBranches(t *testing.T) {
	t.Parallel()

	owner := testModel()
	request, err := modelobject.PublicationRequest(owner, owner.Spec, nil)
	if err != nil {
		t.Fatalf("PublicationRequest() error = %v", err)
	}
	wantErr := errors.New("runtime unavailable")

	cases := []struct {
		name           string
		reconciler     baseReconciler
		mode           runtimeMode
		wantErrSnippet string
	}{
		{
			name:           "unknown mode",
			mode:           runtimeMode("Unknown"),
			wantErrSnippet: "unsupported publication runtime mode",
		},
		{
			name:           "missing source worker runtime",
			mode:           runtimeModeSourceWorker,
			wantErrSnippet: "source worker runtime",
		},
		{
			name: "source worker runtime error",
			reconciler: baseReconciler{
				sourceWorkers: &fakeSourceWorkerRuntime{err: wantErr},
			},
			mode:           runtimeModeSourceWorker,
			wantErrSnippet: wantErr.Error(),
		},
		{
			name:           "missing upload session runtime",
			mode:           runtimeModeUpload,
			wantErrSnippet: "upload session runtime",
		},
		{
			name: "upload session runtime error",
			reconciler: baseReconciler{
				uploadSessions: &fakeUploadSessionRuntime{err: wantErr},
			},
			mode:           runtimeModeUpload,
			wantErrSnippet: wantErr.Error(),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.reconciler.ensureRuntimeObservation(context.Background(), owner, request, tc.mode)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErrSnippet) {
				t.Fatalf("error = %q, want substring %q", got, tc.wantErrSnippet)
			}
		})
	}
}
