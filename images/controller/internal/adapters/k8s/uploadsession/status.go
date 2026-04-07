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
	"errors"
	"fmt"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildUploadStatus(
	artifactURI string,
	service *corev1.Service,
	token string,
	expiresAt metav1.Time,
) modelsv1alpha1.ModelUploadStatus {
	return modelsv1alpha1.ModelUploadStatus{
		ExpiresAt:  &expiresAt,
		Repository: strings.TrimSpace(artifactURI),
		Command:    buildUploadCommand(service.Namespace, service.Name, token),
	}
}

func expiresAtFromSecret(secret *corev1.Secret) (metav1.Time, error) {
	raw := strings.TrimSpace(secret.Annotations["ai-models.deckhouse.io/upload-expires-at"])
	if raw == "" {
		return metav1.Time{}, errors.New("upload session expiry annotation is missing")
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return metav1.Time{}, fmt.Errorf("parse upload session expiry: %w", err)
	}
	return metav1.NewTime(value.UTC()), nil
}

func buildUploadCommand(namespace, serviceName, token string) string {
	return fmt.Sprintf(
		"MODEL_FILE=${MODEL_FILE:?set MODEL_FILE to the local model file or archive path}; kubectl -n %s port-forward service/%s 18444:8444 >/tmp/ai-model-upload-port-forward.log 2>&1 & PF_PID=$!; trap 'kill $PF_PID' EXIT; until curl -fsS http://127.0.0.1:18444/healthz >/dev/null; do sleep 1; done; curl -fsS -X PUT -H 'Authorization: Bearer %s' -H \"X-AI-MODELS-FILENAME: $(basename \"$MODEL_FILE\")\" --data-binary @\"$MODEL_FILE\" http://127.0.0.1:18444/upload",
		namespace,
		serviceName,
		token,
	)
}
