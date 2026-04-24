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
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceDeleteRemovesSessionAndTokenSecrets(t *testing.T) {
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
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected session secret to be deleted, got err=%v", err)
	}
	tokenRef := handle.UploadStatus.TokenSecretRef
	if tokenRef == nil {
		t.Fatal("expected upload token secret ref")
	}
	err = kubeClient.Get(context.Background(), client.ObjectKey{Name: tokenRef.Name, Namespace: tokenRef.Namespace}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected token secret to be deleted, got err=%v", err)
	}
}
