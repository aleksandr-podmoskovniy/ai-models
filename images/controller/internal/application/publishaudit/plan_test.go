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

package publishaudit

import (
	"strings"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPlanPreStatusRecordsUploadRawStaged(t *testing.T) {
	t.Parallel()

	records := PlanPreStatusRecords(true, &cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "artifacts",
			Key:       "raw/1111-2222/model.gguf",
			SizeBytes: 123,
		},
	})
	if got, want := len(records), 1; got != want {
		t.Fatalf("unexpected record count %d", got)
	}
	if got, want := records[0].Reason, ReasonRawStaged; got != want {
		t.Fatalf("unexpected reason %q", got)
	}
	if !strings.Contains(records[0].Message, "s3://artifacts/raw/1111-2222/model.gguf") {
		t.Fatalf("unexpected message %q", records[0].Message)
	}
}

func TestPlanPostStatusRecordsUploadSessionIssued(t *testing.T) {
	t.Parallel()

	expiresAt := metav1.NewTime(time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC))
	records := PlanPostStatusRecords(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelStatus{
			Phase: modelsv1alpha1.ModelPhaseWaitForUpload,
			Upload: &modelsv1alpha1.ModelUploadStatus{
				ExpiresAt: &expiresAt,
			},
		},
		modelsv1alpha1.ModelSourceTypeUpload,
		publicationdomain.Observation{},
	)
	if got, want := len(records), 1; got != want {
		t.Fatalf("unexpected record count %d", got)
	}
	if got, want := records[0].Reason, ReasonUploadSessionIssued; got != want {
		t.Fatalf("unexpected reason %q", got)
	}
}

func TestPlanPostStatusRecordsRemoteStarted(t *testing.T) {
	t.Parallel()

	records := PlanPostStatusRecords(
		modelsv1alpha1.ModelStatus{},
		modelsv1alpha1.ModelStatus{Phase: modelsv1alpha1.ModelPhasePublishing},
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		publicationdomain.Observation{},
	)
	if got, want := len(records), 1; got != want {
		t.Fatalf("unexpected record count %d", got)
	}
	if got, want := records[0].Reason, ReasonRemoteIngestStarted; got != want {
		t.Fatalf("unexpected reason %q", got)
	}
}

func TestPlanPostStatusRecordsPublicationSucceeded(t *testing.T) {
	t.Parallel()

	records := PlanPostStatusRecords(
		modelsv1alpha1.ModelStatus{Phase: modelsv1alpha1.ModelPhasePublishing},
		modelsv1alpha1.ModelStatus{Phase: modelsv1alpha1.ModelPhaseReady},
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		publicationdomain.Observation{
			Snapshot: &publicationdata.Snapshot{
				Artifact: publicationdata.PublishedArtifact{
					URI: "registry.internal.local/model@sha256:deadbeef",
				},
				Source: publicationdata.SourceProvenance{
					Type:           modelsv1alpha1.ModelSourceTypeHuggingFace,
					RawURI:         "s3://artifacts/raw/1111-2222/source-url",
					RawObjectCount: 4,
				},
			},
		},
	)
	if got, want := len(records), 1; got != want {
		t.Fatalf("unexpected record count %d", got)
	}
	if got, want := records[0].Reason, ReasonPublicationSuccess; got != want {
		t.Fatalf("unexpected reason %q", got)
	}
	if !strings.Contains(records[0].Message, "s3://artifacts/raw/1111-2222/source-url") {
		t.Fatalf("unexpected message %q", records[0].Message)
	}
}

func TestPlanPostStatusRecordsPublicationFailed(t *testing.T) {
	t.Parallel()

	records := PlanPostStatusRecords(
		modelsv1alpha1.ModelStatus{Phase: modelsv1alpha1.ModelPhasePublishing},
		modelsv1alpha1.ModelStatus{Phase: modelsv1alpha1.ModelPhaseFailed},
		modelsv1alpha1.ModelSourceTypeHuggingFace,
		publicationdomain.Observation{Message: "scanner policy rejected the payload"},
	)
	if got, want := len(records), 1; got != want {
		t.Fatalf("unexpected record count %d", got)
	}
	if got, want := records[0].Reason, ReasonPublicationFailed; got != want {
		t.Fatalf("unexpected reason %q", got)
	}
}
