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

package ociregistry

import (
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureProjectedAccessCopiesAuthAndCA(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			owner,
			testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-write"),
			testkit.NewOCIRegistryCASecret("d8-ai-models", "ai-models-dmcr-ca"),
		).
		Build()

	projection, err := EnsureProjectedAccess(
		context.Background(),
		kubeClient,
		scheme,
		owner,
		"d8-ai-models",
		types.UID("1111-2222"),
		"ai-models-dmcr-auth-write",
		"ai-models-dmcr-ca",
	)
	if err != nil {
		t.Fatalf("EnsureProjectedAccess() error = %v", err)
	}

	authSecretName, err := resourcenames.OCIRegistryAuthSecretName(types.UID("1111-2222"))
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	if projection.AuthSecretName != authSecretName {
		t.Fatalf("unexpected projected auth secret name %q", projection.AuthSecretName)
	}
	projectedAuth := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: authSecretName, Namespace: "d8-ai-models"}, projectedAuth); err != nil {
		t.Fatalf("Get(projected auth secret) error = %v", err)
	}
	if got, want := string(projectedAuth.Data["username"]), "ai-models"; got != want {
		t.Fatalf("unexpected projected username %q", got)
	}
	if got, want := string(projectedAuth.Data["password"]), "secret"; got != want {
		t.Fatalf("unexpected projected password %q", got)
	}
	if got, want := string(projectedAuth.Data[corev1.DockerConfigJsonKey]), `{"auths":{"registry.internal.local":{"username":"ai-models","password":"secret"}}}`; got != want {
		t.Fatalf("unexpected dockerconfigjson %q", got)
	}

	caSecretName, err := resourcenames.OCIRegistryCASecretName(types.UID("1111-2222"))
	if err != nil {
		t.Fatalf("OCIRegistryCASecretName() error = %v", err)
	}
	if projection.CASecretName != caSecretName {
		t.Fatalf("unexpected projected CA secret name %q", projection.CASecretName)
	}
	projectedCA := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: "d8-ai-models"}, projectedCA); err != nil {
		t.Fatalf("Get(projected ca secret) error = %v", err)
	}
	if got := string(projectedCA.Data["ca.crt"]); got == "" {
		t.Fatal("expected projected ca.crt")
	}
}

func TestEnsureProjectedAccessFromSourceNamespaceCopiesIntoTargetNamespace(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			owner,
			testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
			testkit.NewOCIRegistryCASecret("d8-ai-models", "ai-models-dmcr-ca"),
		).
		Build()

	projection, err := EnsureProjectedAccessFromSourceNamespace(
		context.Background(),
		kubeClient,
		scheme,
		owner,
		"team-a",
		types.UID("1111-2222"),
		"d8-ai-models",
		"ai-models-dmcr-auth-read",
		"ai-models-dmcr-ca",
	)
	if err != nil {
		t.Fatalf("EnsureProjectedAccessFromSourceNamespace() error = %v", err)
	}

	projectedAuth := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: projection.AuthSecretName, Namespace: "team-a"}, projectedAuth); err != nil {
		t.Fatalf("Get(projected auth secret) error = %v", err)
	}
	if got, want := string(projectedAuth.Data["username"]), "ai-models"; got != want {
		t.Fatalf("unexpected projected username %q", got)
	}

	projectedCA := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: projection.CASecretName, Namespace: "team-a"}, projectedCA); err != nil {
		t.Fatalf("Get(projected ca secret) error = %v", err)
	}
	if got := string(projectedCA.Data["ca.crt"]); got == "" {
		t.Fatal("expected projected ca.crt")
	}
}

func TestDeleteProjectedAccessDeletesProjectedSecrets(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	authSecretName, err := resourcenames.OCIRegistryAuthSecretName(types.UID("1111-2222"))
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	caSecretName, err := resourcenames.OCIRegistryCASecretName(types.UID("1111-2222"))
	if err != nil {
		t.Fatalf("OCIRegistryCASecretName() error = %v", err)
	}
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: "d8-ai-models"}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: "d8-ai-models"}},
		).
		Build()

	if err := DeleteProjectedAccess(context.Background(), kubeClient, "d8-ai-models", types.UID("1111-2222")); err != nil {
		t.Fatalf("DeleteProjectedAccess() error = %v", err)
	}

	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: authSecretName, Namespace: "d8-ai-models"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected auth secret to be deleted, got err=%v", err)
	}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: "d8-ai-models"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected CA secret to be deleted, got err=%v", err)
	}
}
