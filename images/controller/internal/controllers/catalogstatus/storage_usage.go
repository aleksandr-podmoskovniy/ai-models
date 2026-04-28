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
	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) syncPublishedStorageUsage(
	ctx context.Context,
	object client.Object,
	status modelsv1alpha1.ModelStatus,
) error {
	if r == nil || r.storageUsage == nil || !r.storageUsage.Enabled() {
		return nil
	}
	if status.Phase != modelsv1alpha1.ModelPhaseReady || status.Artifact == nil || status.Artifact.SizeBytes == nil {
		return nil
	}
	if *status.Artifact.SizeBytes <= 0 {
		return nil
	}
	kind, err := modelobject.KindFor(object)
	if err != nil {
		return err
	}
	return r.storageUsage.CommitPublished(ctx, storagecapacity.PublishedArtifact{
		ID: string(object.GetUID()),
		Owner: storagecapacity.Owner{
			Kind:      kind,
			Name:      object.GetName(),
			Namespace: object.GetNamespace(),
			UID:       string(object.GetUID()),
		},
		SizeBytes: *status.Artifact.SizeBytes,
		UpdatedAt: time.Now().UTC(),
	})
}
