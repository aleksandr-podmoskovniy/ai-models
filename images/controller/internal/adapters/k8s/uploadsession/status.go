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
	"fmt"
	"net/url"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildUploadStatus(
	artifactURI string,
	options Options,
	sessionID string,
	expiresAt metav1.Time,
) modelsv1alpha1.ModelUploadStatus {
	return modelsv1alpha1.ModelUploadStatus{
		ExpiresAt:    &expiresAt,
		Repository:   strings.TrimSpace(artifactURI),
		ExternalURL:  buildExternalUploadURL(options.Gateway.PublicHost, sessionID),
		InClusterURL: buildInClusterUploadURL(options.Gateway.ServiceName, options.Runtime.Namespace, sessionID),
	}
}

func buildAuthorizationHeaderValue(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

func buildInClusterUploadURL(serviceName, namespace, sessionID string) string {
	return buildInClusterUploadURLBase(serviceName, namespace, sessionID)
}

func buildExternalUploadURL(publicHost, sessionID string) string {
	return buildExternalUploadURLBase(publicHost, sessionID)
}

func sessionPath(sessionID string) string {
	return "/v1/upload/" + url.PathEscape(strings.TrimSpace(sessionID))
}

func buildInClusterUploadURLBase(serviceName, namespace, sessionID string) string {
	if strings.TrimSpace(serviceName) == "" || strings.TrimSpace(namespace) == "" {
		return ""
	}
	return fmt.Sprintf(
		"http://%s.%s.svc:%d%s",
		serviceName,
		namespace,
		uploadPort,
		sessionPath(sessionID),
	)
}

func buildExternalUploadURLBase(publicHost, sessionID string) string {
	publicHost = strings.TrimSpace(publicHost)
	if publicHost == "" {
		return ""
	}
	return fmt.Sprintf("https://%s%s", publicHost, sessionPath(sessionID))
}

func tokenFromAuthorizationHeaderValue(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
	if token == "" {
		return "", false
	}
	return token, true
}
