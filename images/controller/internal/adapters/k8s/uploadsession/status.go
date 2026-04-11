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
	token string,
	expiresAt metav1.Time,
) modelsv1alpha1.ModelUploadStatus {
	return modelsv1alpha1.ModelUploadStatus{
		ExpiresAt:    &expiresAt,
		Repository:   strings.TrimSpace(artifactURI),
		ExternalURL:  buildExternalUploadURL(options.Gateway.PublicHost, sessionID, token),
		InClusterURL: buildInClusterUploadURL(options.Gateway.ServiceName, options.Runtime.Namespace, sessionID, token),
	}
}

func buildInClusterUploadURL(serviceName, namespace, sessionID, token string) string {
	base := buildInClusterUploadURLBase(serviceName, namespace, sessionID)
	if base == "" {
		return ""
	}
	return base + "?token=" + url.QueryEscape(strings.TrimSpace(token))
}

func buildExternalUploadURL(publicHost, sessionID, token string) string {
	base := buildExternalUploadURLBase(publicHost, sessionID)
	if base == "" {
		return ""
	}
	return base + "?token=" + url.QueryEscape(strings.TrimSpace(token))
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

func tokenFromPersistedUploadStatus(
	status *modelsv1alpha1.ModelUploadStatus,
	artifactURI string,
	options Options,
	sessionID string,
	expiresAt metav1.Time,
) (string, bool) {
	if status == nil || status.ExpiresAt == nil {
		return "", false
	}
	if strings.TrimSpace(status.Repository) != strings.TrimSpace(artifactURI) {
		return "", false
	}
	if !status.ExpiresAt.Equal(&expiresAt) {
		return "", false
	}

	if token, ok := tokenFromUploadURL(status.InClusterURL, buildInClusterUploadURLBase(options.Gateway.ServiceName, options.Runtime.Namespace, sessionID)); ok {
		return token, true
	}
	if token, ok := tokenFromUploadURL(status.ExternalURL, buildExternalUploadURLBase(options.Gateway.PublicHost, sessionID)); ok {
		return token, true
	}

	return "", false
}

func tokenFromUploadURL(rawURL, expectedBase string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	expectedBase = strings.TrimSpace(expectedBase)
	if rawURL == "" || expectedBase == "" {
		return "", false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	if strings.TrimSpace(parsed.Scheme)+"://"+strings.TrimSpace(parsed.Host)+strings.TrimSpace(parsed.Path) != expectedBase {
		return "", false
	}
	token := strings.TrimSpace(parsed.Query().Get("token"))
	if token == "" {
		return "", false
	}
	return token, true
}
