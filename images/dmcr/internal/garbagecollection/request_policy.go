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

package garbagecollection

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type directUploadSessionClaims struct {
	ObjectKey string `json:"objectKey"`
	UploadID  string `json:"uploadID"`
}

func cleanupPolicyForActiveRequests(configPath string, activeSecrets []corev1.Secret) (cleanupPolicy, error) {
	policy := cleanupPolicy{}
	if len(activeSecrets) == 0 {
		return policy, nil
	}

	var (
		targetTokens []string
	)
	for _, secret := range activeSecrets {
		if strings.TrimSpace(secret.Annotations[directUploadModeAnnotationKey]) != directUploadModeImmediate {
			continue
		}
		if token := strings.TrimSpace(string(secret.Data[directUploadTokenDataKey])); token != "" {
			targetTokens = append(targetTokens, token)
		}
	}
	if len(targetTokens) == 0 {
		return policy, nil
	}

	tokenSecret := []byte(strings.TrimSpace(os.Getenv(directUploadTokenSecretEnv)))
	if len(tokenSecret) == 0 {
		return policy, nil
	}

	storageConfig, err := LoadStorageConfig(configPath)
	if err != nil {
		return cleanupPolicy{}, err
	}

	targetPrefixes := make(map[string]struct{}, len(targetTokens))
	targetMultipartUploads := make(map[directUploadMultipartTarget]struct{}, len(targetTokens))
	for _, token := range targetTokens {
		claims, err := decodeDirectUploadSessionToken(tokenSecret, token)
		if err != nil {
			continue
		}
		prefix, ok := inferDirectUploadPrefix(storageConfig.RootDirectory, claims.ObjectKey)
		if !ok {
			continue
		}
		targetPrefixes[prefix] = struct{}{}
		if strings.TrimSpace(claims.UploadID) != "" {
			targetMultipartUploads[normalizeDirectUploadMultipartTarget(directUploadMultipartTarget{
				ObjectKey: claims.ObjectKey,
				UploadID:  claims.UploadID,
			})] = struct{}{}
		}
	}
	policy.targetDirectUploadPrefixes = targetPrefixes
	policy.targetDirectUploadMultipartUploads = targetMultipartUploads
	return policy, nil
}

func decodeDirectUploadSessionToken(secret []byte, token string) (directUploadSessionClaims, error) {
	left, right, found := strings.Cut(strings.TrimSpace(token), ".")
	if !found || left == "" || right == "" {
		return directUploadSessionClaims{}, fmt.Errorf("direct upload session token is malformed")
	}
	payload, err := base64.RawURLEncoding.DecodeString(left)
	if err != nil {
		return directUploadSessionClaims{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(right)
	if err != nil {
		return directUploadSessionClaims{}, err
	}
	expectedSignature := directUploadSessionTokenSignature(secret, payload)
	if !hmac.Equal(signature, expectedSignature) {
		return directUploadSessionClaims{}, fmt.Errorf("direct upload session token signature mismatch")
	}

	var claims directUploadSessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return directUploadSessionClaims{}, err
	}
	if strings.TrimSpace(claims.ObjectKey) == "" {
		return directUploadSessionClaims{}, fmt.Errorf("direct upload session token is missing object key")
	}
	return claims, nil
}

func directUploadSessionTokenSignature(secret, payload []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}
