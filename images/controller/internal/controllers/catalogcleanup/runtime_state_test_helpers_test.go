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
	"encoding/json"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func requestedGCSecret(namespace string, ownerUID types.UID) *corev1.Secret {
	secret := buildDMCRGCRequestSecret(namespace, cleanupOwner{
		UID:  ownerUID,
		Kind: modelsv1alpha1.ModelKind,
		Name: "deepseek-r1",
	}, "")
	return secret
}

func sourceWorkerStateSecretWithSessionToken(t *testing.T, namespace string, ownerUID types.UID, token string) *corev1.Secret {
	t.Helper()

	name, err := resourcenames.SourceWorkerStateSecretName(ownerUID)
	if err != nil {
		t.Fatalf("SourceWorkerStateSecretName() error = %v", err)
	}
	payload, err := json.Marshal(modelpackports.DirectUploadState{
		Phase: modelpackports.DirectUploadStatePhaseRunning,
		Stage: modelpackports.DirectUploadStateStageUploading,
		CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
			Key:            "model/model.safetensors|application/vnd.ai-models.layer.v1.raw",
			SessionToken:   token,
			PartSizeBytes:  8 << 20,
			TotalSizeBytes: 16 << 20,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"state.json": payload,
		},
	}
}

func assertCleanupCondition(
	t *testing.T,
	kubeClient client.Client,
	object client.Object,
	expectedPhase modelsv1alpha1.ModelPhase,
	expectedReason modelsv1alpha1.ModelConditionReason,
) {
	t.Helper()

	updated := &modelsv1alpha1.Model{}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(object), updated); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if updated.Status.Phase != expectedPhase {
		t.Fatalf("phase = %q, want %q", updated.Status.Phase, expectedPhase)
	}

	var readyCondition *metav1.Condition
	for i := range updated.Status.Conditions {
		if updated.Status.Conditions[i].Type == string(modelsv1alpha1.ModelConditionReady) {
			readyCondition = &updated.Status.Conditions[i]
			break
		}
	}
	if readyCondition == nil {
		t.Fatalf("expected ready condition, got %#v", updated.Status.Conditions)
	}
	if readyCondition.Reason != string(expectedReason) {
		t.Fatalf("condition reason = %q, want %q", readyCondition.Reason, expectedReason)
	}
}
