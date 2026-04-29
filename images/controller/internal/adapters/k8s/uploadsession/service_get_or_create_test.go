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
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
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
	if got, want := handle.Progress, "0%"; got != want {
		t.Fatalf("unexpected progress %q", got)
	}
	if !strings.HasPrefix(handle.UploadStatus.ExternalURL, "https://ai-models.example.com/v1/upload/") {
		t.Fatalf("unexpected external URL %q", handle.UploadStatus.ExternalURL)
	}
	if !strings.Contains(handle.UploadStatus.InClusterURL, "http://ai-models-controller.d8-ai-models.svc:8444/v1/upload/") {
		t.Fatalf("unexpected in-cluster URL %q", handle.UploadStatus.InClusterURL)
	}
	secret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: mustUploadSessionSecret(t, owner.UID).Name, Namespace: "d8-ai-models"}, secret); err != nil {
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
	tokenSecretName, tokenSecretNamespace, err := tokenSecretObjectKey(owner.UID, owner.Namespace, "d8-ai-models")
	if err != nil {
		t.Fatalf("tokenSecretObjectKey() error = %v", err)
	}
	tokenSecret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: tokenSecretName, Namespace: tokenSecretNamespace}, tokenSecret); err != nil {
		t.Fatalf("Get(token secret) error = %v", err)
	}
	if got := string(tokenSecret.Data[TokenSecretAuthorizationHeaderKey]); !strings.HasPrefix(got, "Bearer ") {
		t.Fatalf("unexpected token secret value %q", got)
	}
	rawToken, ok := tokenFromAuthorizationHeaderValue(string(tokenSecret.Data[TokenSecretAuthorizationHeaderKey]))
	if !ok {
		t.Fatalf("unexpected token secret value %q", string(tokenSecret.Data[TokenSecretAuthorizationHeaderKey]))
	}
	assertUploadStatusUsesSecretURL(t, handle.UploadStatus.ExternalURL, rawToken)
	assertUploadStatusUsesSecretURL(t, handle.UploadStatus.InClusterURL, rawToken)
}

func TestServiceGetOrCreateReusesTokenSecretForExistingSession(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")
	uploadStatus := testUploadStatus()
	owner.Status.Upload = &uploadStatus

	secret := mustUploadSessionSecret(t, owner.UID)
	tokenSecret := mustUploadTokenSecret(t, owner.UID, owner.Namespace, "existing-token")

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner, secret, tokenSecret).
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
	if got, want := handle.UploadStatus.InClusterURL, "http://ai-models-controller.d8-ai-models.svc:8444/v1/upload/ai-model-upload-auth-1111-2222/existing-token"; got != want {
		t.Fatalf("unexpected in-cluster URL %q, want %q", got, want)
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
	tokenSecret := mustUploadTokenSecret(t, owner.UID, owner.Namespace, "existing-token")

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner, secret, tokenSecret).
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
	tokenSecretName, tokenSecretNamespace, err := tokenSecretObjectKey(owner.UID, owner.Namespace, "d8-ai-models")
	if err != nil {
		t.Fatalf("tokenSecretObjectKey() error = %v", err)
	}
	updatedTokenSecret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: tokenSecretName, Namespace: tokenSecretNamespace}, updatedTokenSecret); err != nil {
		t.Fatalf("Get(updated token secret) error = %v", err)
	}
	if got := string(updatedTokenSecret.Data[TokenSecretAuthorizationHeaderKey]); got == "Bearer existing-token" {
		t.Fatalf("expected recreated session to rotate upload token secret value, got %q", got)
	}
	rawToken, ok := tokenFromAuthorizationHeaderValue(string(updatedTokenSecret.Data[TokenSecretAuthorizationHeaderKey]))
	if !ok {
		t.Fatalf("unexpected token secret value %q", string(updatedTokenSecret.Data[TokenSecretAuthorizationHeaderKey]))
	}
	if rawToken == "existing-token" {
		t.Fatalf("expected rotated raw token, got %q", rawToken)
	}
	assertUploadStatusUsesSecretURL(t, handle.UploadStatus.InClusterURL, rawToken)
}

func TestServiceGetOrCreateRefreshesMultipartProgressFromStaging(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")

	secret := mustUploadSessionSecret(t, owner.UID)
	if err := uploadsessionstate.SaveMultipartSecret(secret, uploadsessionruntime.SessionState{
		UploadID: "upload-1",
		Key:      "raw/1111-2222/model.safetensors",
		FileName: "model.safetensors",
	}); err != nil {
		t.Fatalf("SaveMultipartSecret() error = %v", err)
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner, secret).
		Build()

	options := testUploadOptions()
	options.StagingBucket = "ai-models"
	options.StagingClient = &fakeMultipartStager{
		listMultipartUploadParts: func(_ context.Context, input uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
			if input.Bucket != "ai-models" || input.Key != "raw/1111-2222/model.safetensors" || input.UploadID != "upload-1" {
				t.Fatalf("unexpected list parts input %#v", input)
			}
			return []uploadstagingports.UploadedPart{
				{PartNumber: 1, ETag: "etag-1", SizeBytes: 64},
				{PartNumber: 2, ETag: "etag-2", SizeBytes: 64},
			}, nil
		},
	}

	service, err := NewService(kubeClient, scheme, options)
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
	if got, want := handle.Progress, "100%"; got != want {
		t.Fatalf("unexpected progress %q", got)
	}

	updatedSecret := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(secret), updatedSecret); err != nil {
		t.Fatalf("Get(updated secret) error = %v", err)
	}
	session, err := uploadsessionstate.SessionFromSecret(updatedSecret)
	if err != nil {
		t.Fatalf("SessionFromSecret() error = %v", err)
	}
	if session.Multipart == nil || len(session.Multipart.UploadedParts) != 2 {
		t.Fatalf("unexpected persisted multipart state %#v", session.Multipart)
	}
	if got, want := session.Multipart.UploadedParts[0].SizeBytes+session.Multipart.UploadedParts[1].SizeBytes, int64(128); got != want {
		t.Fatalf("unexpected persisted uploaded size %d", got)
	}
}

func assertUploadStatusUsesSecretURL(t *testing.T, uploadURL string, rawToken string) {
	t.Helper()
	if !strings.HasSuffix(uploadURL, "/"+rawToken) {
		t.Fatalf("expected upload URL %q to end with secret token %q", uploadURL, rawToken)
	}
	if strings.Contains(uploadURL, "Bearer") {
		t.Fatalf("upload URL must not contain bearer header prefix: %q", uploadURL)
	}
}
