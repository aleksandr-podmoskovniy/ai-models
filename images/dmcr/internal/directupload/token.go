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

package directupload

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

type sessionTokenClaims struct {
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
	UploadID   string `json:"uploadID"`
}

func encodeSessionToken(secret []byte, claims sessionTokenClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signature := tokenSignature(secret, payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func decodeSessionToken(secret []byte, token string) (sessionTokenClaims, error) {
	left, right, found := strings.Cut(strings.TrimSpace(token), ".")
	if !found || left == "" || right == "" {
		return sessionTokenClaims{}, errors.New("direct upload session token is malformed")
	}
	payload, err := base64.RawURLEncoding.DecodeString(left)
	if err != nil {
		return sessionTokenClaims{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(right)
	if err != nil {
		return sessionTokenClaims{}, err
	}
	expectedSignature := tokenSignature(secret, payload)
	if !hmac.Equal(signature, expectedSignature) {
		return sessionTokenClaims{}, errors.New("direct upload session token signature mismatch")
	}
	var claims sessionTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return sessionTokenClaims{}, err
	}
	return claims, nil
}

func tokenSignature(secret, payload []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}
