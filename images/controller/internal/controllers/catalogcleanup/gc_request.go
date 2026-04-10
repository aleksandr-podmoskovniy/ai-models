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

package catalogcleanup

import (
	"context"
	"fmt"
	"time"

	deletionapp "github.com/deckhouse/ai-models/controller/internal/application/deletion"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dmcrGCRequestLabelKey     = "ai-models.deckhouse.io/dmcr-gc-request"
	dmcrGCRequestLabelValue   = "true"
	dmcrGCSwitchAnnotationKey = "ai-models.deckhouse.io/dmcr-gc-switch"
	dmcrGCDoneAnnotationKey   = "ai-models.deckhouse.io/dmcr-gc-done"
	dmcrGCRequestNamePrefix   = "dmcr-gc-"
	dmcrGCRequestTimestampRFC = time.RFC3339Nano
)

func dmcrGCRequestSecretName(ownerUID types.UID) string {
	suffix, err := resourcenames.OwnerSuffix(ownerUID)
	if err != nil {
		return fmt.Sprintf("%sinvalid", dmcrGCRequestNamePrefix)
	}
	return fmt.Sprintf("%s%s", dmcrGCRequestNamePrefix, suffix)
}

func buildDMCRGCRequestSecret(namespace string, owner cleanupJobOwner) *corev1.Secret {
	annotations := map[string]string{
		dmcrGCSwitchAnnotationKey: time.Now().UTC().Format(dmcrGCRequestTimestampRFC),
	}
	labels := map[string]string{
		dmcrGCRequestLabelKey:               dmcrGCRequestLabelValue,
		"ai-models.deckhouse.io/owner-kind": resourcenames.TruncateLabelValue(owner.Kind),
		"ai-models.deckhouse.io/owner-name": resourcenames.TruncateLabelValue(owner.Name),
		"ai-models.deckhouse.io/owner-uid":  resourcenames.TruncateLabelValue(string(owner.UID)),
	}
	if owner.Namespace != "" {
		labels["ai-models.deckhouse.io/owner-namespace"] = resourcenames.TruncateLabelValue(owner.Namespace)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dmcrGCRequestSecretName(owner.UID),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func observeDMCRGCRequestState(secret *corev1.Secret) deletionapp.GarbageCollectionState {
	if secret == nil {
		return deletionapp.GarbageCollectionStateMissing
	}
	if secret.Annotations[dmcrGCSwitchAnnotationKey] != "" {
		return deletionapp.GarbageCollectionStateRequested
	}
	if secret.Annotations[dmcrGCDoneAnnotationKey] != "" {
		return deletionapp.GarbageCollectionStateComplete
	}
	return deletionapp.GarbageCollectionStateMissing
}

func (r *baseReconciler) ensureGarbageCollectionRequest(ctx context.Context, owner cleanupJobOwner) error {
	key := client.ObjectKey{
		Namespace: r.options.CleanupJob.Namespace,
		Name:      dmcrGCRequestSecretName(owner.UID),
	}
	var existing corev1.Secret
	switch err := r.client.Get(ctx, key, &existing); {
	case apierrors.IsNotFound(err):
		return r.client.Create(ctx, buildDMCRGCRequestSecret(r.options.CleanupJob.Namespace, owner))
	case err != nil:
		return err
	default:
		if existing.Labels == nil {
			existing.Labels = make(map[string]string, 1)
		}
		existing.Labels[dmcrGCRequestLabelKey] = dmcrGCRequestLabelValue
		if existing.Annotations == nil {
			existing.Annotations = make(map[string]string, 2)
		}
		delete(existing.Annotations, dmcrGCDoneAnnotationKey)
		existing.Annotations[dmcrGCSwitchAnnotationKey] = time.Now().UTC().Format(dmcrGCRequestTimestampRFC)
		return r.client.Update(ctx, &existing)
	}
}

func (r *baseReconciler) deleteGarbageCollectionRequest(ctx context.Context, ownerUID types.UID) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.options.CleanupJob.Namespace,
			Name:      dmcrGCRequestSecretName(ownerUID),
		},
	}
	if err := r.client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
