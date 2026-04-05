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

package ownedresource

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateOrGetCreatesControlledObject(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}

	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "publication", Namespace: "d8-ai-models", UID: "operation-uid"}}
	desired := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "upload", Namespace: "d8-ai-models"}}

	created, err := CreateOrGet(context.Background(), kubeClient, scheme, owner, desired)
	if err != nil {
		t.Fatalf("CreateOrGet() error = %v", err)
	}
	if !created {
		t.Fatal("expected resource to be created")
	}
	if len(desired.OwnerReferences) != 1 || desired.OwnerReferences[0].UID != owner.UID {
		t.Fatalf("unexpected owner references %#v", desired.OwnerReferences)
	}

	stored := &corev1.Service{}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(desired), stored); err != nil {
		t.Fatalf("Get(stored service) error = %v", err)
	}
	if len(stored.OwnerReferences) != 1 || stored.OwnerReferences[0].UID != owner.UID {
		t.Fatalf("unexpected stored owner references %#v", stored.OwnerReferences)
	}
}

func TestCreateOrGetLoadsExistingObject(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}

	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "upload-auth", Namespace: "d8-ai-models"},
		Data:       map[string][]byte{"token": []byte("existing-token")},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "publication", Namespace: "d8-ai-models", UID: "operation-uid"}}
	desired := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "upload-auth", Namespace: "d8-ai-models"}}

	created, err := CreateOrGet(context.Background(), kubeClient, scheme, owner, desired)
	if err != nil {
		t.Fatalf("CreateOrGet() error = %v", err)
	}
	if created {
		t.Fatal("expected existing object to be reused")
	}
	if got := string(desired.Data["token"]); got != "existing-token" {
		t.Fatalf("unexpected existing token %q", got)
	}
}
