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

package nodecacheintent

import (
	"context"
	"testing"

	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceApplyConfigMapCreatesAndUpdatesDesiredProjection(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	service, err := NewService(kubeClient)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	firstIntents := []intentcontract.ArtifactIntent{{
		ArtifactURI: "oci://example/model-a",
		Digest:      "sha256:a",
	}}
	if err := service.ApplyConfigMap(context.Background(), "d8-ai-models", "worker-a", firstIntents); err != nil {
		t.Fatalf("ApplyConfigMap(create) error = %v", err)
	}

	configMap := &corev1.ConfigMap{}
	name, err := DesiredConfigMap("d8-ai-models", "worker-a", nil)
	if err != nil {
		t.Fatalf("DesiredConfigMap() error = %v", err)
	}
	if err := kubeClient.Get(context.Background(), clientObjectKey(name), configMap); err != nil {
		t.Fatalf("Get(configmap) error = %v", err)
	}
	decoded, err := intentcontract.DecodeIntents([]byte(configMap.Data[intentcontract.DataKey]))
	if err != nil {
		t.Fatalf("DecodeIntents() error = %v", err)
	}
	if got, want := len(decoded), 1; got != want {
		t.Fatalf("intent count = %d, want %d", got, want)
	}

	secondIntents := []intentcontract.ArtifactIntent{
		{ArtifactURI: "oci://example/model-a", Digest: "sha256:a"},
		{ArtifactURI: "oci://example/model-b", Digest: "sha256:b"},
	}
	if err := service.ApplyConfigMap(context.Background(), "d8-ai-models", "worker-a", secondIntents); err != nil {
		t.Fatalf("ApplyConfigMap(update) error = %v", err)
	}

	if err := kubeClient.Get(context.Background(), clientObjectKey(name), configMap); err != nil {
		t.Fatalf("Get(updated configmap) error = %v", err)
	}
	decoded, err = intentcontract.DecodeIntents([]byte(configMap.Data[intentcontract.DataKey]))
	if err != nil {
		t.Fatalf("DecodeIntents(updated) error = %v", err)
	}
	if got, want := len(decoded), 2; got != want {
		t.Fatalf("updated intent count = %d, want %d", got, want)
	}
}

func TestServiceDeleteConfigMapIgnoresMissingObject(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ai-models-node-cache-intent-worker-a",
			Namespace: "d8-ai-models",
		},
	}).Build()
	service, err := NewService(kubeClient)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.DeleteConfigMap(context.Background(), "d8-ai-models", "worker-a"); err != nil {
		t.Fatalf("DeleteConfigMap(existing) error = %v", err)
	}
	if err := service.DeleteConfigMap(context.Background(), "d8-ai-models", "worker-a"); err != nil {
		t.Fatalf("DeleteConfigMap(missing) error = %v", err)
	}

	configMap := &corev1.ConfigMap{}
	err = kubeClient.Get(context.Background(), types.NamespacedName{
		Namespace: "d8-ai-models",
		Name:      "ai-models-node-cache-intent-worker-a",
	}, configMap)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected configmap deletion, got error %v", err)
	}
}

func clientObjectKey(object clientObject) types.NamespacedName {
	return types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()}
}

type clientObject interface {
	GetNamespace() string
	GetName() string
}
