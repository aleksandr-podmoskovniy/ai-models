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
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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
		ExpiresAt:                &expiresAt,
		Repository:               "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1-upload/1111-2222:published",
		ExternalURL:              "https://ai-models.example.com/v1/upload/ai-model-upload-auth-1111-2222",
		InClusterURL:             "http://ai-models-controller.d8-ai-models.svc:8444/v1/upload/ai-model-upload-auth-1111-2222",
		AuthorizationHeaderValue: "Bearer existing-token",
	}
}
