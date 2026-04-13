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

package uploadsessionstate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
	corev1 "k8s.io/api/core/v1"
)

var ErrTokenHashMissing = errors.New("upload session token hash must not be empty")

func SetToken(secret *corev1.Secret, rawToken string) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	if len(secret.Data) == 0 {
		secret.Data = make(map[string][]byte, 1)
	}
	return setToken(secretDataAccessor{data: secret.Data}, rawToken)
}

func tokenHashFromSecret(secret *corev1.Secret) (string, error) {
	tokenHash := strings.TrimSpace(string(secret.Data[tokenHashKey]))
	if tokenHash == "" {
		return "", ErrTokenHashMissing
	}
	if !isValidTokenHash(tokenHash) {
		return "", fmt.Errorf("upload session token hash must be a 64-character lowercase hex string")
	}
	return tokenHash, nil
}

type secretDataAccessor struct {
	data map[string][]byte
}

func setToken(target secretDataAccessor, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return errors.New("upload session token must not be empty")
	}
	if target.data == nil {
		target.data = make(map[string][]byte, 1)
	}
	target.data[tokenHashKey] = []byte(uploadsessiontoken.Hash(rawToken))
	return nil
}

func isValidTokenHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		default:
			return false
		}
	}
	return true
}
