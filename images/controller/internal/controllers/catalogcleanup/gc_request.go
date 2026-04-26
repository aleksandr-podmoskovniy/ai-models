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
	dmcrGCPhaseAnnotationKey     = "ai.deckhouse.io/dmcr-gc-phase"
	dmcrGCPhaseQueued            = "queued"
	dmcrGCPhaseDone              = "done"
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

func buildDMCRGCRequestSecret(namespace string, owner cleanupOwner, directUploadSessionToken string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dmcrGCRequestSecretName(owner.UID),
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
	queueDMCRGCRequestSecret(secret, owner, directUploadSessionToken)
	return secret
}

func queueDMCRGCRequestSecret(secret *corev1.Secret, owner cleanupOwner, directUploadSessionToken string) {
	requestedAt := time.Now().UTC().Format(dmcrGCRequestTimestampRFC)
	secret.Labels = mergeLabels(secret.Labels, garbageCollectionRequestLabels(owner))
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string, 3)
	}
	secret.Annotations[dmcrGCRequestedAnnotationKey] = requestedAt
	delete(secret.Annotations, dmcrGCSwitchAnnotationKey)
	secret.Annotations[dmcrGCPhaseAnnotationKey] = dmcrGCPhaseQueued

	token := strings.TrimSpace(directUploadSessionToken)
	if token != "" {
		secret.Annotations[dmcrGCDirectUploadModeKey] = dmcrGCDirectUploadModeFast
		if secret.Data == nil {
			secret.Data = make(map[string][]byte, 1)
		}
		secret.Data[dmcrGCDirectUploadTokenKey] = []byte(token)
		return
	}
	delete(secret.Annotations, dmcrGCDirectUploadModeKey)
	if len(secret.Data) == 0 {
		return
	}
	delete(secret.Data, dmcrGCDirectUploadTokenKey)
	if len(secret.Data) == 0 {
		secret.Data = nil
	}
}

func observeDMCRGCRequestState(secret *corev1.Secret) deletionapp.GarbageCollectionState {
	if secret == nil {
		return deletionapp.GarbageCollectionStateMissing
	}
	if strings.TrimSpace(secret.Annotations[dmcrGCPhaseAnnotationKey]) == dmcrGCPhaseDone {
		return deletionapp.GarbageCollectionStateComplete
	}
	if secret.Annotations[dmcrGCSwitchAnnotationKey] != "" {
		return deletionapp.GarbageCollectionStateRequested
	}
	if secret.Annotations[dmcrGCRequestedAnnotationKey] != "" {
		return deletionapp.GarbageCollectionStateQueued
	}
	return deletionapp.GarbageCollectionStateMissing
}

func (r *baseReconciler) ensureGarbageCollectionRequest(ctx context.Context, owner cleanupOwner) error {
	sessionToken, err := r.directUploadSessionTokenSnapshot(ctx, owner.UID)
	if err != nil {
		return err
	}

	key := garbageCollectionRequestKey(r.options.Cleanup.Namespace, owner.UID)
	var existing corev1.Secret
	switch err := r.client.Get(ctx, key, &existing); {
	case apierrors.IsNotFound(err):
		return r.client.Create(ctx, buildDMCRGCRequestSecret(r.options.Cleanup.Namespace, owner, sessionToken))
	case err != nil:
		return err
	default:
		queueDMCRGCRequestSecret(&existing, owner, sessionToken)
		return r.client.Update(ctx, &existing)
	}
}

func (r *baseReconciler) directUploadSessionTokenSnapshot(ctx context.Context, ownerUID types.UID) (string, error) {
	name, err := resourcenames.SourceWorkerStateSecretName(ownerUID)
	if err != nil {
		return "", err
	}

	var secret corev1.Secret
	key := types.NamespacedName{Namespace: r.options.Cleanup.Namespace, Name: name}
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
	key := garbageCollectionRequestKey(r.options.Cleanup.Namespace, ownerUID)
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
