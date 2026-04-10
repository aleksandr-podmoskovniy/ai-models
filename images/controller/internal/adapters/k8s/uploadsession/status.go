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
	"net/url"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildUploadStatus(
	artifactURI string,
	service *corev1.Service,
	ingress *networkingv1.Ingress,
	token string,
	expiresAt metav1.Time,
) modelsv1alpha1.ModelUploadStatus {
	return modelsv1alpha1.ModelUploadStatus{
		ExpiresAt:    &expiresAt,
		Repository:   strings.TrimSpace(artifactURI),
		ExternalURL:  buildExternalUploadURL(ingress, token),
		InClusterURL: buildInClusterUploadURL(service, token),
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

func buildInClusterUploadURL(service *corev1.Service, token string) string {
	if service == nil {
		return ""
	}
	port := int32(uploadPort)
	if len(service.Spec.Ports) > 0 && service.Spec.Ports[0].Port > 0 {
		port = service.Spec.Ports[0].Port
	}
	return fmt.Sprintf(
		"http://%s.%s.svc:%d%s",
		service.Name,
		service.Namespace,
		port,
		uploadSessionPath(token),
	)
}

func buildExternalUploadURL(ingress *networkingv1.Ingress, token string) string {
	if ingress == nil {
		return ""
	}
	host := strings.TrimSpace(firstIngressHost(ingress))
	if host == "" {
		return ""
	}
	scheme := "http"
	if len(ingress.Spec.TLS) > 0 {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, uploadSessionPath(token))
}

func uploadSessionPath(token string) string {
	return "/upload/" + url.PathEscape(strings.TrimSpace(token))
}

func firstIngressHost(ingress *networkingv1.Ingress) string {
	if ingress == nil || len(ingress.Spec.Rules) == 0 {
		return ""
	}
	return ingress.Spec.Rules[0].Host
}
