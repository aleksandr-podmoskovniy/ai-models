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

package publicationops

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestConfigMapStoreFailPersistsFailureState(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	operation, err := NewConfigMap("d8-ai-models", uploadRequest(modelsv1alpha1.ModelUploadFormatHuggingFaceDirectory))
	if err != nil {
		t.Fatalf("NewConfigMap() error = %v", err)
	}
	operation.Data[uploadDataKey] = `{"command":"curl","repository":"repo","expiresAt":"2026-04-03T10:00:00Z"}`
	operation.Data[resultDataKey] = `{"stale":"result"}`

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(operation).
		Build()

	reconciler := &Reconciler{client: kubeClient}
	if err := reconciler.failOperation(context.Background(), operation, "boom"); err != nil {
		t.Fatalf("failOperation() error = %v", err)
	}

	var updated corev1.ConfigMap
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(operation), &updated); err != nil {
		t.Fatalf("Get(operation) error = %v", err)
	}
	status := StatusFromConfigMap(&updated)
	if got, want := status.Phase, PhaseFailed; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if got, want := status.Message, "boom"; got != want {
		t.Fatalf("unexpected message %q", got)
	}
	if _, found := updated.Data[uploadDataKey]; found {
		t.Fatal("upload payload must be cleared")
	}
	if _, found := updated.Data[resultDataKey]; found {
		t.Fatal("result payload must be cleared")
	}
}
