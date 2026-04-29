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

package workloaddelivery

import (
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createLegacyProjectedAccess(t *testing.T, kubeClient client.Client, namespace string, ownerUID types.UID) {
	t.Helper()

	for _, name := range legacyProjectedSecretNames(t, ownerUID) {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		if err := kubeClient.Create(context.Background(), secret); err != nil {
			t.Fatalf("Create(legacy projected secret %s/%s) error = %v", namespace, name, err)
		}
	}
}

func assertLegacyProjectedAuthSecretAbsent(t *testing.T, kubeClient client.Client, namespace string, ownerUID types.UID) {
	t.Helper()

	for _, name := range legacyProjectedSecretNames(t, ownerUID)[:2] {
		assertLegacySecretAbsent(t, kubeClient, namespace, name)
	}
}

func assertLegacyRuntimeImagePullSecretAbsent(t *testing.T, kubeClient client.Client, namespace string, ownerUID types.UID) {
	t.Helper()

	name, err := resourcenames.RuntimeImagePullSecretName(ownerUID)
	if err != nil {
		t.Fatalf("RuntimeImagePullSecretName() error = %v", err)
	}
	assertLegacySecretAbsent(t, kubeClient, namespace, name)
}

func legacyProjectedSecretNames(t *testing.T, ownerUID types.UID) []string {
	t.Helper()

	authName, err := resourcenames.OCIRegistryAuthSecretName(ownerUID)
	if err != nil {
		t.Fatalf("OCIRegistryAuthSecretName() error = %v", err)
	}
	caName, err := resourcenames.OCIRegistryCASecretName(ownerUID)
	if err != nil {
		t.Fatalf("OCIRegistryCASecretName() error = %v", err)
	}
	pullName, err := resourcenames.RuntimeImagePullSecretName(ownerUID)
	if err != nil {
		t.Fatalf("RuntimeImagePullSecretName() error = %v", err)
	}
	return []string{authName, caName, pullName}
}

func assertLegacySecretAbsent(t *testing.T, kubeClient client.Client, namespace string, name string) {
	t.Helper()

	err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected legacy projected secret %s/%s to be absent, got err=%v", namespace, name, err)
	}
}
