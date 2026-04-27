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
	targetTokens := collectDirectUploadImmediateTokens(activeSecrets)
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

	return directUploadCleanupPolicyForTokens(storageConfig.RootDirectory, tokenSecret, targetTokens), nil
}

func collectDirectUploadImmediateTokens(activeSecrets []corev1.Secret) []string {
	tokens := make([]string, 0, len(activeSecrets))
	for _, secret := range activeSecrets {
		if strings.TrimSpace(secret.Annotations[directUploadModeAnnotationKey]) != directUploadModeImmediate {
			continue
		}
		if token := strings.TrimSpace(string(secret.Data[directUploadTokenDataKey])); token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func directUploadCleanupPolicyForTokens(rootDirectory string, tokenSecret []byte, tokens []string) cleanupPolicy {
	targetPrefixes := make(map[string]struct{}, len(tokens))
	targetMultipartUploads := make(map[directUploadMultipartTarget]struct{}, len(tokens))
	for _, token := range tokens {
		claims, err := decodeDirectUploadSessionToken(tokenSecret, token)
		if err != nil {
			continue
		}
		prefix, ok := inferDirectUploadPrefix(rootDirectory, claims.ObjectKey)
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
	return cleanupPolicy{
		targetDirectUploadPrefixes:         targetPrefixes,
		targetDirectUploadMultipartUploads: targetMultipartUploads,
	}
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
