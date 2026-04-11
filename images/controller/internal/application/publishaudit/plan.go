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
	"fmt"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	"github.com/deckhouse/ai-models/controller/internal/ports/auditsink"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
)

const (
	ReasonUploadSessionIssued = "UploadSessionIssued"
	ReasonRemoteIngestStarted = "RemoteIngestStarted"
	ReasonRawStaged           = "RawStaged"
	ReasonPublicationSuccess  = "PublicationSucceeded"
	ReasonPublicationFailed   = "PublicationFailed"
)

func PlanPreStatusRecords(handleUpdated bool, handle *cleanuphandle.Handle) []auditsink.Record {
	if !handleUpdated || handle == nil || handle.Kind != cleanuphandle.KindUploadStaging || handle.UploadStaging == nil {
		return nil
	}

	record := auditsink.Record{
		Type:    corev1.EventTypeNormal,
		Reason:  ReasonRawStaged,
		Message: fmt.Sprintf("controller staged raw upload bytes at %s", rawObjectURI(handle.UploadStaging.Bucket, handle.UploadStaging.Key)),
	}
	if handle.UploadStaging.SizeBytes > 0 {
		record.Message += fmt.Sprintf(" (%d bytes)", handle.UploadStaging.SizeBytes)
	}
	return []auditsink.Record{record}
}

func PlanPostStatusRecords(
	current modelsv1alpha1.ModelStatus,
	desired modelsv1alpha1.ModelStatus,
	sourceType modelsv1alpha1.ModelSourceType,
	observation publicationdomain.Observation,
) []auditsink.Record {
	records := make([]auditsink.Record, 0, 2)

	switch {
	case desired.Phase == modelsv1alpha1.ModelPhaseWaitForUpload && current.Phase != desired.Phase && desired.Upload != nil:
		records = append(records, auditsink.Record{
			Type:    corev1.EventTypeNormal,
			Reason:  ReasonUploadSessionIssued,
			Message: uploadSessionIssuedMessage(desired.Upload),
		})
	case desired.Phase == modelsv1alpha1.ModelPhasePublishing && current.Phase != desired.Phase && sourceType != modelsv1alpha1.ModelSourceTypeUpload:
		records = append(records, auditsink.Record{
			Type:    corev1.EventTypeNormal,
			Reason:  ReasonRemoteIngestStarted,
			Message: fmt.Sprintf("controller started remote raw ingest for source type %s", sourceType),
		})
	}

	switch {
	case desired.Phase == modelsv1alpha1.ModelPhaseReady && current.Phase != desired.Phase && observation.Snapshot != nil:
		records = append(records, auditsink.Record{
			Type:    corev1.EventTypeNormal,
			Reason:  ReasonPublicationSuccess,
			Message: publicationSucceededMessage(*observation.Snapshot),
		})
	case desired.Phase == modelsv1alpha1.ModelPhaseFailed && current.Phase != desired.Phase:
		records = append(records, auditsink.Record{
			Type:    corev1.EventTypeWarning,
			Reason:  ReasonPublicationFailed,
			Message: failureMessage(observation.Message),
		})
	}

	return records
}

func uploadSessionIssuedMessage(status *modelsv1alpha1.ModelUploadStatus) string {
	if status == nil || status.ExpiresAt == nil {
		return "controller issued an upload session"
	}
	return fmt.Sprintf("controller issued an upload session; expires at %s", status.ExpiresAt.Time.UTC().Format(time.RFC3339))
}

func publicationSucceededMessage(snapshot publicationdata.Snapshot) string {
	message := fmt.Sprintf("controller published artifact %s", strings.TrimSpace(snapshot.Artifact.URI))
	if rawURI := strings.TrimSpace(snapshot.Source.RawURI); rawURI != "" {
		message += fmt.Sprintf(" from raw %s", rawURI)
		if snapshot.Source.RawObjectCount > 0 {
			message += fmt.Sprintf(" (%d object(s))", snapshot.Source.RawObjectCount)
		}
	}
	return message
}

func failureMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "controller failed to publish the model artifact"
	}
	return message
}

func rawObjectURI(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", strings.TrimSpace(bucket), strings.Trim(strings.TrimSpace(key), "/"))
}
