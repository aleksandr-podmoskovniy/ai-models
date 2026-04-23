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
	"strings"
	"time"

	directuploadstate "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/directuploadstate"
	deletionapp "github.com/deckhouse/ai-models/controller/internal/application/deletion"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	dmcrGCRequestLabelKey        = "ai.deckhouse.io/dmcr-gc-request"
	dmcrGCRequestLabelValue      = "true"
	dmcrGCRequestedAnnotationKey = "ai.deckhouse.io/dmcr-gc-requested-at"
	dmcrGCSwitchAnnotationKey    = "ai.deckhouse.io/dmcr-gc-switch"
	dmcrGCDoneAnnotationKey      = "ai.deckhouse.io/dmcr-gc-done"
	dmcrGCDirectUploadModeKey    = "ai.deckhouse.io/dmcr-gc-direct-upload-mode"
	dmcrGCDirectUploadModeFast   = "immediate-orphan-cleanup"
	dmcrGCDirectUploadTokenKey   = "direct-upload-session-token"
	dmcrGCRequestNamePrefix      = "dmcr-gc-"
	dmcrGCRequestTimestampRFC    = time.RFC3339Nano
)

func dmcrGCRequestSecretName(ownerUID types.UID) string {
	suffix, err := resourcenames.OwnerSuffix(ownerUID)
	if err != nil {
		return fmt.Sprintf("%sinvalid", dmcrGCRequestNamePrefix)
	}
	return fmt.Sprintf("%s%s", dmcrGCRequestNamePrefix, suffix)
}

func buildDMCRGCRequestSecret(namespace string, owner cleanupJobOwner, directUploadSessionToken string) *corev1.Secret {
	requestedAt := time.Now().UTC().Format(dmcrGCRequestTimestampRFC)
	annotations := map[string]string{
		dmcrGCRequestedAnnotationKey: requestedAt,
		dmcrGCSwitchAnnotationKey:    requestedAt,
		dmcrGCDirectUploadModeKey:    dmcrGCDirectUploadModeFast,
	}
	data := map[string][]byte(nil)
	if token := strings.TrimSpace(directUploadSessionToken); token != "" {
		data = map[string][]byte{
			dmcrGCDirectUploadTokenKey: []byte(token),
		}
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dmcrGCRequestSecretName(owner.UID),
			Namespace:   namespace,
			Labels:      garbageCollectionRequestLabels(owner),
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}

func observeDMCRGCRequestState(secret *corev1.Secret) deletionapp.GarbageCollectionState {
	if secret == nil {
		return deletionapp.GarbageCollectionStateMissing
	}
	if secret.Annotations[dmcrGCSwitchAnnotationKey] != "" {
		return deletionapp.GarbageCollectionStateRequested
	}
	if secret.Annotations[dmcrGCRequestedAnnotationKey] != "" {
		return deletionapp.GarbageCollectionStateQueued
	}
	if secret.Annotations[dmcrGCDoneAnnotationKey] != "" {
		return deletionapp.GarbageCollectionStateComplete
	}
	return deletionapp.GarbageCollectionStateMissing
}

func (r *baseReconciler) ensureGarbageCollectionRequest(ctx context.Context, owner cleanupJobOwner) error {
	sessionToken, err := r.directUploadSessionTokenSnapshot(ctx, owner.UID)
	if err != nil {
		return err
	}

	key := garbageCollectionRequestKey(r.options.CleanupJob.Namespace, owner.UID)
	var existing corev1.Secret
	switch err := r.client.Get(ctx, key, &existing); {
	case apierrors.IsNotFound(err):
		return r.client.Create(ctx, buildDMCRGCRequestSecret(r.options.CleanupJob.Namespace, owner, sessionToken))
	case err != nil:
		return err
	default:
		existing.Labels = mergeLabels(existing.Labels, garbageCollectionRequestLabels(owner))
		if existing.Annotations == nil {
			existing.Annotations = make(map[string]string, 3)
		}
		requestedAt := time.Now().UTC().Format(dmcrGCRequestTimestampRFC)
		existing.Annotations[dmcrGCRequestedAnnotationKey] = requestedAt
		existing.Annotations[dmcrGCSwitchAnnotationKey] = requestedAt
		existing.Annotations[dmcrGCDirectUploadModeKey] = dmcrGCDirectUploadModeFast
		delete(existing.Annotations, dmcrGCDoneAnnotationKey)
		if token := strings.TrimSpace(sessionToken); token != "" {
			if existing.Data == nil {
				existing.Data = make(map[string][]byte, 1)
			}
			existing.Data[dmcrGCDirectUploadTokenKey] = []byte(token)
		} else if len(existing.Data) > 0 {
			delete(existing.Data, dmcrGCDirectUploadTokenKey)
			if len(existing.Data) == 0 {
				existing.Data = nil
			}
		}
		return r.client.Update(ctx, &existing)
	}
}

func (r *baseReconciler) directUploadSessionTokenSnapshot(ctx context.Context, ownerUID types.UID) (string, error) {
	name, err := resourcenames.SourceWorkerStateSecretName(ownerUID)
	if err != nil {
		return "", err
	}

	var secret corev1.Secret
	key := types.NamespacedName{Namespace: r.options.CleanupJob.Namespace, Name: name}
	switch err := r.client.Get(ctx, key, &secret); {
	case apierrors.IsNotFound(err):
		return "", nil
	case err != nil:
		return "", err
	}

	state, err := directuploadstate.StateFromSecret(&secret)
	if err != nil {
		return "", nil
	}
	if state.CurrentLayer == nil {
		return "", nil
	}
	return strings.TrimSpace(state.CurrentLayer.SessionToken), nil
}

func (r *baseReconciler) deleteGarbageCollectionRequest(ctx context.Context, ownerUID types.UID) error {
	key := garbageCollectionRequestKey(r.options.CleanupJob.Namespace, ownerUID)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
	}
	if err := r.client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
