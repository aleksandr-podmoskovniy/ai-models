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

package sync_artifacts_secrets

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/module-sdk/pkg"
	"github.com/deckhouse/module-sdk/pkg/registry"

	"hooks/pkg/settings"
)

const (
	secretsSnapshotName = "artifacts-source-secrets"

	credentialsSecretNamePath = "aiModels.artifacts.credentialsSecretName"
	caSecretNamePath          = "aiModels.artifacts.caSecretName"

	internalArtifactsPath                    = "aiModels.internal.artifacts"
	internalSyncedCredentialsSecretNamePath  = internalArtifactsPath + ".syncedCredentialsSecretName"
	internalMountedCASecretNamePath          = internalArtifactsPath + ".mountedCASecretName"
	syncedCredentialsSecretName              = "ai-models-artifacts"
	syncedCASecretName                       = "ai-models-artifacts-ca"
	sourceSecretSnapshotFilter               = `{"name": .metadata.name, "data": {"accessKey": .data.accessKey, "secretKey": .data.secretKey, "ca.crt": .data."ca.crt"}}`
	sourceSecretAnnotationNamespace          = "ai-models.deckhouse.io/source-secret-namespace"
	sourceSecretAnnotationName               = "ai-models.deckhouse.io/source-secret-name"
)

var _ = registry.RegisterFunc(config, Reconcile)

var config = &pkg.HookConfig{
	OnBeforeHelm: &pkg.OrderedConfig{Order: 12},
	Kubernetes: []pkg.KubernetesConfig{
		{
			Name:       secretsSnapshotName,
			APIVersion: "v1",
			Kind:       "Secret",
			JqFilter:   sourceSecretSnapshotFilter,
			NamespaceSelector: &pkg.NamespaceSelector{
				NameSelector: &pkg.NameSelector{
					MatchNames: []string{settings.DeckhouseNamespace},
				},
			},
		},
	},
}

type sourceSecretSnapshot struct {
	Name string            `json:"name"`
	Data map[string][]byte `json:"data"`
}

func Reconcile(_ context.Context, input *pkg.HookInput) error {
	input.Values.Set(internalSyncedCredentialsSecretNamePath, syncedCredentialsSecretName)

	credentialsSourceName := strings.TrimSpace(input.Values.Get(credentialsSecretNamePath).String())
	caSourceName := strings.TrimSpace(input.Values.Get(caSecretNamePath).String())

	if credentialsSourceName == "" {
		input.Values.Remove(internalMountedCASecretNamePath)
		input.PatchCollector.DeleteInBackground("v1", "Secret", settings.ModuleNamespace, syncedCredentialsSecretName)
		input.PatchCollector.DeleteInBackground("v1", "Secret", settings.ModuleNamespace, syncedCASecretName)
		return nil
	}

	secretsByName, err := sourceSecretsByName(input)
	if err != nil {
		return err
	}

	credentialsSource, ok := secretsByName[credentialsSourceName]
	if !ok {
		return fmt.Errorf("artifacts credentials secret %s/%s not found", settings.DeckhouseNamespace, credentialsSourceName)
	}

	accessKey, err := requiredSecretData(credentialsSource, "accessKey")
	if err != nil {
		return fmt.Errorf("artifacts credentials secret %s/%s: %w", settings.DeckhouseNamespace, credentialsSourceName, err)
	}
	secretKey, err := requiredSecretData(credentialsSource, "secretKey")
	if err != nil {
		return fmt.Errorf("artifacts credentials secret %s/%s: %w", settings.DeckhouseNamespace, credentialsSourceName, err)
	}

	credentialsData := map[string][]byte{
		"accessKey": accessKey,
		"secretKey": secretKey,
	}
	if ca := optionalSecretData(credentialsSource, "ca.crt"); len(ca) > 0 {
		credentialsData["ca.crt"] = ca
	}

	input.PatchCollector.CreateOrUpdate(moduleOwnedSecret(
		syncedCredentialsSecretName,
		credentialsSourceName,
		credentialsData,
	))

	mountedCASecretName := ""

	switch {
	case caSourceName == "":
		if len(optionalSecretData(credentialsSource, "ca.crt")) > 0 {
			mountedCASecretName = syncedCredentialsSecretName
		}
		input.PatchCollector.DeleteInBackground("v1", "Secret", settings.ModuleNamespace, syncedCASecretName)
	case caSourceName == credentialsSourceName:
		if len(optionalSecretData(credentialsSource, "ca.crt")) == 0 {
			return fmt.Errorf("artifacts CA secret %s/%s must contain ca.crt", settings.DeckhouseNamespace, caSourceName)
		}
		mountedCASecretName = syncedCredentialsSecretName
		input.PatchCollector.DeleteInBackground("v1", "Secret", settings.ModuleNamespace, syncedCASecretName)
	default:
		caSource, ok := secretsByName[caSourceName]
		if !ok {
			return fmt.Errorf("artifacts CA secret %s/%s not found", settings.DeckhouseNamespace, caSourceName)
		}

		caData, err := requiredSecretData(caSource, "ca.crt")
		if err != nil {
			return fmt.Errorf("artifacts CA secret %s/%s: %w", settings.DeckhouseNamespace, caSourceName, err)
		}

		input.PatchCollector.CreateOrUpdate(moduleOwnedSecret(
			syncedCASecretName,
			caSourceName,
			map[string][]byte{"ca.crt": caData},
		))
		mountedCASecretName = syncedCASecretName
	}

	if mountedCASecretName == "" {
		input.Values.Remove(internalMountedCASecretNamePath)
	} else {
		input.Values.Set(internalMountedCASecretNamePath, mountedCASecretName)
	}

	return nil
}

func sourceSecretsByName(input *pkg.HookInput) (map[string]sourceSecretSnapshot, error) {
	snapshots := input.Snapshots.Get(secretsSnapshotName)
	secrets := make(map[string]sourceSecretSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		var secret sourceSecretSnapshot
		if err := snapshot.UnmarshalTo(&secret); err != nil {
			return nil, fmt.Errorf("unmarshal source secret snapshot: %w", err)
		}
		secrets[secret.Name] = secret
	}
	return secrets, nil
}

func requiredSecretData(secret sourceSecretSnapshot, key string) ([]byte, error) {
	value := optionalSecretData(secret, key)
	if len(value) == 0 {
		return nil, fmt.Errorf("must contain non-empty %s", key)
	}
	return value, nil
}

func optionalSecretData(secret sourceSecretSnapshot, key string) []byte {
	return bytes.TrimSpace(secret.Data[key])
}

func moduleOwnedSecret(name, sourceName string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: settings.ModuleNamespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "ai-models",
			},
			Annotations: map[string]string{
				sourceSecretAnnotationNamespace: settings.DeckhouseNamespace,
				sourceSecretAnnotationName:      sourceName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}
