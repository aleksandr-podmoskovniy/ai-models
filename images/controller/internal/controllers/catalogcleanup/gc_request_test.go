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
	"testing"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBuildDMCRGCRequestSecretIncludesSharedOwnerLabels(t *testing.T) {
	t.Parallel()

	secret := buildDMCRGCRequestSecret("d8-ai-models", cleanupJobOwner{
		UID:       types.UID("1111-2222"),
		Kind:      "Model",
		Name:      "deepseek-r1",
		Namespace: "team-a",
	}, "")

	if got, want := secret.Labels[resourcenames.AppNameLabelKey], garbageCollectionRequestAppName; got != want {
		t.Fatalf("unexpected app label %q", got)
	}
	if got, want := secret.Labels[dmcrGCRequestLabelKey], dmcrGCRequestLabelValue; got != want {
		t.Fatalf("unexpected request label %q", got)
	}
	if got, want := secret.Labels[resourcenames.OwnerKindLabelKey], "Model"; got != want {
		t.Fatalf("unexpected owner-kind label %q", got)
	}
	if got, want := secret.Labels[resourcenames.OwnerNamespaceLabelKey], "team-a"; got != want {
		t.Fatalf("unexpected owner-namespace label %q", got)
	}
	if secret.Annotations[dmcrGCRequestedAnnotationKey] == "" {
		t.Fatal("expected queued-request annotation on garbage-collection request secret")
	}
	if secret.Annotations[dmcrGCSwitchAnnotationKey] == "" {
		t.Fatal("expected delete-triggered request to be armed immediately")
	}
	if secret.Annotations[dmcrGCRequestedAnnotationKey] != secret.Annotations[dmcrGCSwitchAnnotationKey] {
		t.Fatalf("expected requested-at and switch timestamps to match, got %#v", secret.Annotations)
	}
	if got := string(secret.Data[dmcrGCDirectUploadTokenKey]); got != "" {
		t.Fatalf("unexpected direct-upload session token payload %q", got)
	}
}

func TestEnsureGarbageCollectionRequestRefreshesMetadataOnExistingSecret(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	owner := cleanupJobOwner{
		UID:       model.GetUID(),
		Kind:      "Model",
		Name:      "deepseek-r1",
		Namespace: "team-a",
	}
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "d8-ai-models",
			Name:      dmcrGCRequestSecretName(owner.UID),
			Labels: map[string]string{
				"extra": "keep",
			},
			Annotations: map[string]string{
				dmcrGCDoneAnnotationKey: time.Now().UTC().Format(dmcrGCRequestTimestampRFC),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			dmcrGCDirectUploadTokenKey: []byte("old-token"),
			"extra":                    []byte("keep"),
		},
	}
	reconciler, kubeClient := newModelReconciler(t, model, existing)

	if err := reconciler.ensureGarbageCollectionRequest(context.Background(), owner); err != nil {
		t.Fatalf("ensureGarbageCollectionRequest() error = %v", err)
	}

	var updated corev1.Secret
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(existing), &updated); err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if got, want := updated.Labels["extra"], "keep"; got != want {
		t.Fatalf("unexpected preserved label %q", got)
	}
	if got, want := updated.Labels[resourcenames.AppNameLabelKey], garbageCollectionRequestAppName; got != want {
		t.Fatalf("unexpected app label %q", got)
	}
	if got, want := updated.Labels[resourcenames.OwnerUIDLabelKey], string(owner.UID); got != want {
		t.Fatalf("unexpected owner UID label %q", got)
	}
	if updated.Annotations[dmcrGCDoneAnnotationKey] != "" {
		t.Fatalf("expected done annotation to be removed, got %#v", updated.Annotations)
	}
	if updated.Annotations[dmcrGCRequestedAnnotationKey] == "" {
		t.Fatalf("expected queued-request annotation to be set, got %#v", updated.Annotations)
	}
	if updated.Annotations[dmcrGCSwitchAnnotationKey] == "" {
		t.Fatalf("expected active switch annotation to be set, got %#v", updated.Annotations)
	}
	if updated.Annotations[dmcrGCRequestedAnnotationKey] != updated.Annotations[dmcrGCSwitchAnnotationKey] {
		t.Fatalf("expected requested-at and switch timestamps to match, got %#v", updated.Annotations)
	}
	if _, found := updated.Data[dmcrGCDirectUploadTokenKey]; found {
		t.Fatalf("expected stale direct-upload token to be removed, got %#v", updated.Data)
	}
	if got, want := string(updated.Data["extra"]), "keep"; got != want {
		t.Fatalf("unexpected preserved data %q", got)
	}
}

func TestEnsureGarbageCollectionRequestSnapshotsCurrentDirectUploadSessionToken(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	owner := cleanupJobOwner{
		UID:       model.GetUID(),
		Kind:      "Model",
		Name:      "deepseek-r1",
		Namespace: "team-a",
	}
	const sessionToken = "session-token-1"
	stateSecret := sourceWorkerStateSecretWithSessionToken(t, "d8-ai-models", model.GetUID(), sessionToken)
	reconciler, kubeClient := newModelReconciler(t, model, stateSecret)

	if err := reconciler.ensureGarbageCollectionRequest(context.Background(), owner); err != nil {
		t.Fatalf("ensureGarbageCollectionRequest() error = %v", err)
	}

	var request corev1.Secret
	key := client.ObjectKey{Namespace: "d8-ai-models", Name: dmcrGCRequestSecretName(owner.UID)}
	if err := kubeClient.Get(context.Background(), key, &request); err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if got, want := string(request.Data[dmcrGCDirectUploadTokenKey]), sessionToken; got != want {
		t.Fatalf("session token payload = %q, want %q", got, want)
	}
}
