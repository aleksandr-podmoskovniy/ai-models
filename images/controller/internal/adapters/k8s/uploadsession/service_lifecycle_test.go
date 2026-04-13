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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
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

func TestServiceGetOrCreateReusesPersistedUploadURLForExistingSession(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")
	uploadStatus := testUploadStatus()
	owner.Status.Upload = &uploadStatus

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
}

func TestServiceGetOrCreateRecreatesStaleSecretWithoutTokenHash(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")
	uploadStatus := testUploadStatus()
	owner.Status.Upload = &uploadStatus

	secret := mustUploadSessionSecret(t, owner.UID)
	delete(secret.Data, "tokenHash")
	secret.Data["token"] = []byte("stale-token")

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
	if !created {
		t.Fatal("expected stale upload session secret to be recreated")
	}
	if handle == nil {
		t.Fatal("expected upload session handle")
	}
	if handle.UploadStatus.InClusterURL == owner.Status.Upload.InClusterURL {
		t.Fatalf("expected recreated session to rotate upload URL token, got %q", handle.UploadStatus.InClusterURL)
	}
	if !strings.Contains(handle.UploadStatus.InClusterURL, "/v1/upload/ai-model-upload-auth-1111-2222?token=") {
		t.Fatalf("unexpected recreated in-cluster URL %q", handle.UploadStatus.InClusterURL)
	}

	updatedSecret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(secret), updatedSecret); err != nil {
		t.Fatalf("Get(updated secret) error = %v", err)
	}
	if _, found := updatedSecret.Data["token"]; found {
		t.Fatalf("stale raw token must not survive secret recreation: %#v", updatedSecret.Data)
	}
	if _, found := updatedSecret.Data["tokenHash"]; !found {
		t.Fatalf("expected recreated secret to persist tokenHash, got %#v", updatedSecret.Data)
	}
}
