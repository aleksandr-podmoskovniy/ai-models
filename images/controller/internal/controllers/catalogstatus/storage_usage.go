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
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) syncPublishedStorageUsage(
	ctx context.Context,
	object client.Object,
	status modelsv1alpha1.ModelStatus,
	observation publicationdomain.Observation,
) error {
	if r == nil || r.storageUsage == nil || !r.storageUsage.Enabled() {
		return nil
	}
	if status.Phase == modelsv1alpha1.ModelPhaseFailed {
		return r.releasePublicationStorageReservations(ctx, object)
	}
	if status.Phase != modelsv1alpha1.ModelPhaseReady || status.Artifact == nil || status.Artifact.SizeBytes == nil {
		return nil
	}
	sizeBytes := *status.Artifact.SizeBytes
	if observation.Snapshot != nil && observation.Snapshot.Source.RawSizeBytes > 0 {
		sizeBytes += observation.Snapshot.Source.RawSizeBytes
	}
	if sizeBytes <= 0 {
		return nil
	}
	kind, err := modelobject.KindFor(object)
	if err != nil {
		return err
	}
	ownerUID := string(object.GetUID())
	uploadReservationID, err := resourcenames.UploadSessionSecretName(object.GetUID())
	if err != nil {
		return err
	}
	return r.storageUsage.CommitPublishedReplacingReservations(ctx, []string{
		ownerUID,
		uploadReservationID,
	}, storagecapacity.PublishedArtifact{
		ID: ownerUID,
		Owner: storagecapacity.Owner{
			Kind:      kind,
			Name:      object.GetName(),
			Namespace: object.GetNamespace(),
			UID:       ownerUID,
		},
		SizeBytes: sizeBytes,
		UpdatedAt: time.Now().UTC(),
	})
}

func (r *baseReconciler) releasePublicationStorageReservations(ctx context.Context, object client.Object) error {
	ownerUID := string(object.GetUID())
	if err := r.storageUsage.ReleaseReservation(ctx, ownerUID); err != nil {
		return err
	}
	uploadReservationID, err := resourcenames.UploadSessionSecretName(object.GetUID())
	if err != nil {
		return err
	}
	return r.storageUsage.ReleaseReservation(ctx, uploadReservationID)
}
