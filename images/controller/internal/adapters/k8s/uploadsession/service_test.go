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

package uploadsession

import (
	"context"
	"strings"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceGetOrCreateCreatesSharedGatewaySessionSecret(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")
	owner.Name = "deepseek-r1-upload"

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner).
		Build()

	service, err := NewService(kubeClient, scheme, testUploadOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	request := testUploadRequest()
	request.Owner.UID = owner.UID
	request.Owner.Name = owner.Name
	request.Identity.Name = owner.Name

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected upload session secret to be created")
	}
	if handle == nil || handle.WorkerName == "" {
		t.Fatalf("unexpected upload session handle %#v", handle)
	}
	if !strings.HasPrefix(handle.UploadStatus.ExternalURL, "https://ai-models.example.com/v1/upload/") {
		t.Fatalf("unexpected external URL %q", handle.UploadStatus.ExternalURL)
	}
	if !strings.Contains(handle.UploadStatus.InClusterURL, "http://ai-models-controller.d8-ai-models.svc:8444/v1/upload/") {
		t.Fatalf("unexpected in-cluster URL %q", handle.UploadStatus.InClusterURL)
	}

	secretName, err := resourcenames.UploadSessionSecretName(owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionSecretName() error = %v", err)
	}
	secret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: secretName, Namespace: "d8-ai-models"}, secret); err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	session, err := uploadsessionstate.SessionFromSecret(secret)
	if err != nil {
		t.Fatalf("SessionFromSecret() error = %v", err)
	}
	if session.Phase != uploadsessionstate.PhaseIssued {
		t.Fatalf("unexpected session phase %q", session.Phase)
	}
	if _, found := secret.Data["token"]; found {
		t.Fatalf("raw upload token must not be stored in session secret: %#v", secret.Data)
	}
}

func TestServiceGetOrCreateProjectsUploadedAndFailedSessionState(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		mutate         func(t *testing.T, secret *corev1.Secret)
		wantPhase      corev1.PodPhase
		wantTermSubstr string
	}{
		{
			name: "uploaded",
			mutate: func(t *testing.T, secret *corev1.Secret) {
				t.Helper()
				handle := cleanuphandle.Handle{
					Kind: cleanuphandle.KindUploadStaging,
					UploadStaging: &cleanuphandle.UploadStagingHandle{
						Bucket:    "ai-models",
						Key:       "raw/1111-2222/model.gguf",
						FileName:  "model.gguf",
						SizeBytes: 128,
					},
				}
				encoded, err := cleanuphandle.Encode(handle)
				if err != nil {
					t.Fatalf("Encode() error = %v", err)
				}
				secret.Data["phase"] = []byte(string(uploadsessionstate.PhaseUploaded))
				secret.Data["stagedHandle"] = []byte(encoded)
			},
			wantPhase:      corev1.PodSucceeded,
			wantTermSubstr: "\"kind\":\"UploadStaging\"",
		},
		{
			name: "failed",
			mutate: func(t *testing.T, secret *corev1.Secret) {
				t.Helper()
				secret.Data["phase"] = []byte(string(uploadsessionstate.PhaseFailed))
				secret.Data["failureMessage"] = []byte("upload failed")
			},
			wantPhase:      corev1.PodFailed,
			wantTermSubstr: "upload failed",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheme := testkit.NewScheme(t)
			owner := testkit.NewUploadModel()
			owner.UID = types.UID("1111-2222")

			secret := mustUploadSessionSecret(t, owner.UID)
			tc.mutate(t, secret)

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(owner, secret).
				Build()

			service, err := NewService(kubeClient, scheme, testUploadOptions())
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			request := testUploadRequest()
			request.Owner.UID = owner.UID

			handle, created, err := service.GetOrCreate(context.Background(), owner, request)
			if err != nil {
				t.Fatalf("GetOrCreate() error = %v", err)
			}
			if created {
				t.Fatal("expected existing session to be reused")
			}
			if handle == nil || handle.Phase != tc.wantPhase {
				t.Fatalf("unexpected handle %#v", handle)
			}
			if !strings.Contains(handle.TerminationMessage, tc.wantTermSubstr) {
				t.Fatalf("unexpected termination message %q", handle.TerminationMessage)
			}
		})
	}
}

func TestServiceDeleteRemovesOnlySessionSecret(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")

	secret := mustUploadSessionSecret(t, owner.UID)
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner, secret).
		Build()

	service, err := NewService(kubeClient, scheme, testUploadOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	request := testUploadRequest()
	request.Owner.UID = owner.UID

	handle, _, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if err := handle.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	err = kubeClient.Get(context.Background(), client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
	if client.IgnoreNotFound(err) != nil || err == nil {
		t.Fatalf("expected secret to be deleted, got err=%v", err)
	}
}

func TestServiceGetOrCreateMigratesLegacyTokenStorageAndReusesPersistedUploadURL(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")
	uploadStatus := testUploadStatus()
	owner.Status.Upload = &uploadStatus

	secret := mustUploadSessionSecret(t, owner.UID)
	secret.Data["token"] = []byte("existing-token")
	delete(secret.Data, "tokenHash")

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner, secret).
		Build()

	service, err := NewService(kubeClient, scheme, testUploadOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	request := testUploadRequest()
	request.Owner.UID = owner.UID

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if created {
		t.Fatal("expected existing session to be reused")
	}
	if handle.UploadStatus.InClusterURL != owner.Status.Upload.InClusterURL {
		t.Fatalf("expected persisted in-cluster URL to be reused, got %q", handle.UploadStatus.InClusterURL)
	}

	updatedSecret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(secret), updatedSecret); err != nil {
		t.Fatalf("Get(updated secret) error = %v", err)
	}
	if _, found := updatedSecret.Data["token"]; found {
		t.Fatalf("raw upload token must be removed from secret after migration: %#v", updatedSecret.Data)
	}
	if strings.TrimSpace(string(updatedSecret.Data["tokenHash"])) == "" {
		t.Fatalf("expected token hash to be persisted after migration: %#v", updatedSecret.Data)
	}
}

func TestServiceSyncsControllerOwnedSessionPhases(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")

	for _, tc := range []struct {
		name          string
		mark          func(context.Context, *Service, types.UID) error
		wantPhase     uploadsessionstate.Phase
		wantMessage   string
		wantHandleSet bool
	}{
		{
			name: "publishing",
			mark: func(ctx context.Context, service *Service, ownerUID types.UID) error {
				return service.MarkPublishing(ctx, ownerUID)
			},
			wantPhase:     uploadsessionstate.PhasePublishing,
			wantHandleSet: true,
		},
		{
			name: "completed",
			mark: func(ctx context.Context, service *Service, ownerUID types.UID) error {
				return service.MarkCompleted(ctx, ownerUID)
			},
			wantPhase: uploadsessionstate.PhaseCompleted,
		},
		{
			name: "failed",
			mark: func(ctx context.Context, service *Service, ownerUID types.UID) error {
				return service.MarkFailed(ctx, ownerUID, "publish failed")
			},
			wantPhase:     uploadsessionstate.PhaseFailed,
			wantMessage:   "publish failed",
			wantHandleSet: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			secret := mustUploadSessionSecret(t, owner.UID)
			handle := cleanuphandle.Handle{
				Kind: cleanuphandle.KindUploadStaging,
				UploadStaging: &cleanuphandle.UploadStagingHandle{
					Bucket:    "ai-models",
					Key:       "raw/1111-2222/model.gguf",
					FileName:  "model.gguf",
					SizeBytes: 128,
				},
			}
			if err := uploadsessionstate.MarkUploadedSecret(secret, handle); err != nil {
				t.Fatalf("MarkUploadedSecret() error = %v", err)
			}

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(owner.DeepCopy(), secret).
				Build()

			service, err := NewService(kubeClient, scheme, testUploadOptions())
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}

			if err := tc.mark(context.Background(), service, owner.UID); err != nil {
				t.Fatalf("phase sync error = %v", err)
			}

			updatedSecret := &corev1.Secret{}
			if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(secret), updatedSecret); err != nil {
				t.Fatalf("Get(updated secret) error = %v", err)
			}
			session, err := uploadsessionstate.SessionFromSecret(updatedSecret)
			if err != nil {
				t.Fatalf("SessionFromSecret() error = %v", err)
			}
			if session.Phase != tc.wantPhase {
				t.Fatalf("unexpected session phase %q", session.Phase)
			}
			if session.FailureMessage != tc.wantMessage {
				t.Fatalf("unexpected failure message %q", session.FailureMessage)
			}
			if got := session.StagedHandle != nil; got != tc.wantHandleSet {
				t.Fatalf("unexpected staged handle presence %v", got)
			}
		})
	}
}

func mustUploadSessionSecret(t *testing.T, ownerUID types.UID) *corev1.Secret {
	t.Helper()
	name, err := resourcenames.UploadSessionSecretName(ownerUID)
	if err != nil {
		t.Fatalf("UploadSessionSecretName() error = %v", err)
	}
	stagingPrefix, err := resourcenames.UploadStagingObjectPrefix(ownerUID)
	if err != nil {
		t.Fatalf("UploadStagingObjectPrefix() error = %v", err)
	}
	secret, err := uploadsessionstate.NewSecret(uploadsessionstate.SessionSpec{
		Name:              name,
		Namespace:         "d8-ai-models",
		Token:             "existing-token",
		ExpectedSizeBytes: 128,
		StagingKeyPrefix:  stagingPrefix,
		ExpiresAt:         time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}
	secret.Annotations[uploadsessionstate.ExpiresAtAnnotationKey] = metav1.NewTime(time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC)).Format(time.RFC3339)
	return secret
}

func testUploadStatus() modelsv1alpha1.ModelUploadStatus {
	expiresAt := metav1.NewTime(time.Date(2030, 4, 10, 13, 0, 0, 0, time.UTC))
	return modelsv1alpha1.ModelUploadStatus{
		ExpiresAt:    &expiresAt,
		Repository:   "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1-upload/1111-2222:published",
		ExternalURL:  "https://ai-models.example.com/v1/upload/ai-model-upload-auth-1111-2222?token=existing-token",
		InClusterURL: "http://ai-models-controller.d8-ai-models.svc:8444/v1/upload/ai-model-upload-auth-1111-2222?token=existing-token",
	}
}
