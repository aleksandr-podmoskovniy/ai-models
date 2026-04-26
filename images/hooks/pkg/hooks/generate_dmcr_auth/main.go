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

package generate_dmcr_auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/module-sdk/pkg"
	"github.com/deckhouse/module-sdk/pkg/registry"

	"hooks/pkg/settings"
)

const (
	authSecretSnapshotName = "dmcr-auth-secrets"

	authSecretName      = "ai-models-dmcr-auth"
	writeAuthSecretName = "ai-models-dmcr-auth-write"
	readAuthSecretName  = "ai-models-dmcr-auth-read"

	authValuesPath          = "aiModels.internal.dmcr.auth"
	writePasswordValuesPath = authValuesPath + ".writePassword"
	readPasswordValuesPath  = authValuesPath + ".readPassword"
	writeHtpasswdValuesPath = authValuesPath + ".writeHtpasswd"
	readHtpasswdValuesPath  = authValuesPath + ".readHtpasswd"
	saltValuesPath          = authValuesPath + ".salt"

	writeUsername = "ai-models"
	readUsername  = "ai-models-reader"

	passwordLength = 40
	saltLength     = 64
)

type secretSnapshot struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

type authState struct {
	WritePassword string
	ReadPassword  string
	WriteHtpasswd string
	ReadHtpasswd  string
	Salt          string
}

var _ = registry.RegisterFunc(config, Reconcile)

var config = &pkg.HookConfig{
	OnBeforeHelm: &pkg.OrderedConfig{Order: 5},
	Kubernetes: []pkg.KubernetesConfig{
		{
			Name:       authSecretSnapshotName,
			APIVersion: "v1",
			Kind:       "Secret",
			JqFilter:   `{"name": .metadata.name, "data": .data}`,
			NameSelector: &pkg.NameSelector{
				MatchNames: []string{authSecretName, writeAuthSecretName, readAuthSecretName},
			},
			NamespaceSelector: &pkg.NamespaceSelector{
				NameSelector: &pkg.NameSelector{
					MatchNames: []string{settings.ModuleNamespace},
				},
			},
			ExecuteHookOnSynchronization: ptr.To(false),
		},
	},
	Queue: fmt.Sprintf("modules/%s", settings.ModuleName),
}

func Reconcile(_ context.Context, input *pkg.HookInput) error {
	existing, err := stateFromSecrets(input)
	if err != nil {
		return err
	}
	values := stateFromValues(input)

	state, err := desiredState(existing, values)
	if err != nil {
		return err
	}
	setState(input, state)
	return nil
}

func desiredState(existing, values authState) (authState, error) {
	state := authState{
		WritePassword: firstNonEmpty(existing.WritePassword, values.WritePassword),
		ReadPassword:  firstNonEmpty(existing.ReadPassword, values.ReadPassword),
		WriteHtpasswd: firstNonEmpty(existing.WriteHtpasswd, values.WriteHtpasswd),
		ReadHtpasswd:  firstNonEmpty(existing.ReadHtpasswd, values.ReadHtpasswd),
		Salt:          firstNonEmpty(existing.Salt, values.Salt),
	}

	var err error
	if state.WritePassword == "" {
		state.WritePassword, err = prefixedAlphaNum(passwordLength)
		if err != nil {
			return authState{}, fmt.Errorf("generate write password: %w", err)
		}
	}
	if state.ReadPassword == "" {
		state.ReadPassword, err = prefixedAlphaNum(passwordLength)
		if err != nil {
			return authState{}, fmt.Errorf("generate read password: %w", err)
		}
	}
	if state.Salt == "" {
		state.Salt, err = alphaNum(saltLength)
		if err != nil {
			return authState{}, fmt.Errorf("generate salt: %w", err)
		}
	}
	if !validateHtpasswd(writeUsername, state.WritePassword, state.WriteHtpasswd) {
		state.WriteHtpasswd, err = generateHtpasswd(writeUsername, state.WritePassword)
		if err != nil {
			return authState{}, fmt.Errorf("generate write htpasswd: %w", err)
		}
	}
	if !validateHtpasswd(readUsername, state.ReadPassword, state.ReadHtpasswd) {
		state.ReadHtpasswd, err = generateHtpasswd(readUsername, state.ReadPassword)
		if err != nil {
			return authState{}, fmt.Errorf("generate read htpasswd: %w", err)
		}
	}

	return state, nil
}

func stateFromSecrets(input *pkg.HookInput) (authState, error) {
	snapshots := input.Snapshots.Get(authSecretSnapshotName)
	secrets := make(map[string]map[string]string, len(snapshots))
	for _, snapshot := range snapshots {
		var secret secretSnapshot
		if err := snapshot.UnmarshalTo(&secret); err != nil {
			return authState{}, fmt.Errorf("unmarshal DMCR auth secret snapshot: %w", err)
		}
		secrets[secret.Name] = secret.Data
	}

	authData := secrets[authSecretName]
	writeData := secrets[writeAuthSecretName]
	readData := secrets[readAuthSecretName]

	return authState{
		WritePassword: firstNonEmpty(secretValue(authData, "write.password"), secretValue(writeData, "password")),
		ReadPassword:  firstNonEmpty(secretValue(authData, "read.password"), secretValue(readData, "password")),
		WriteHtpasswd: secretValue(authData, "write.htpasswd"),
		ReadHtpasswd:  secretValue(authData, "read.htpasswd"),
		Salt:          secretValue(authData, "salt"),
	}, nil
}

func stateFromValues(input *pkg.HookInput) authState {
	return authState{
		WritePassword: strings.TrimSpace(input.Values.Get(writePasswordValuesPath).String()),
		ReadPassword:  strings.TrimSpace(input.Values.Get(readPasswordValuesPath).String()),
		WriteHtpasswd: strings.TrimSpace(input.Values.Get(writeHtpasswdValuesPath).String()),
		ReadHtpasswd:  strings.TrimSpace(input.Values.Get(readHtpasswdValuesPath).String()),
		Salt:          strings.TrimSpace(input.Values.Get(saltValuesPath).String()),
	}
}

func setState(input *pkg.HookInput, state authState) {
	input.Values.Set(writePasswordValuesPath, state.WritePassword)
	input.Values.Set(readPasswordValuesPath, state.ReadPassword)
	input.Values.Set(writeHtpasswdValuesPath, state.WriteHtpasswd)
	input.Values.Set(readHtpasswdValuesPath, state.ReadHtpasswd)
	input.Values.Set(saltValuesPath, state.Salt)
}

func secretValue(data map[string]string, key string) string {
	value := strings.TrimSpace(data[key])
	if value == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(decoded))
}

func generateHtpasswd(username, password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", username, string(hashed)), nil
}

func validateHtpasswd(username, password, entry string) bool {
	storedUser, hash, ok := strings.Cut(strings.TrimSpace(entry), ":")
	if !ok || storedUser != username {
		return false
	}
	hash = strings.TrimSpace(hash)
	if strings.HasPrefix(hash, "$2y$") {
		hash = "$2a$" + strings.TrimPrefix(hash, "$2y$")
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func prefixedAlphaNum(length int) (string, error) {
	const prefix = "A1a"
	tail, err := alphaNum(length - len(prefix))
	if err != nil {
		return "", err
	}
	return prefix + tail, nil
}

func alphaNum(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var builder strings.Builder
	builder.Grow(length)

	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < length; i++ {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[index.Int64()])
	}
	return builder.String(), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
