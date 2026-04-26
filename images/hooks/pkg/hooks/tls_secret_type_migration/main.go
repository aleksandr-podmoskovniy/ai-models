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

package tls_secret_type_migration

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/module-sdk/pkg"
	"github.com/deckhouse/module-sdk/pkg/registry"

	"hooks/pkg/settings"
)

const (
	tlsSecretsSnapshotName   = "legacy-tls-secret-types"
	tlsSecretSnapshotFilter  = `{"name": .metadata.name, "type": .type}`
	webhookTLSSecretName     = "ai-models-controller-webhook-tls"
	dmcrTLSSecretName        = "ai-models-dmcr-tls"
	tlsSecretMigrationOrder  = 6
	secretAPIVersion         = "v1"
	secretKind               = "Secret"
	expectedKubernetesTLSRaw = string(corev1.SecretTypeTLS)
)

var managedTLSSecretNames = []string{
	webhookTLSSecretName,
	dmcrTLSSecretName,
}

var _ = registry.RegisterFunc(config, Reconcile)

// Common TLS hooks run at order 5 and load existing cert material into values.
// This hook must run after them so Helm can recreate a legacy Secret without
// rotating the certificate.
var config = &pkg.HookConfig{
	OnBeforeHelm: &pkg.OrderedConfig{Order: tlsSecretMigrationOrder},
	Kubernetes: []pkg.KubernetesConfig{
		{
			Name:       tlsSecretsSnapshotName,
			APIVersion: secretAPIVersion,
			Kind:       secretKind,
			JqFilter:   tlsSecretSnapshotFilter,
			NamespaceSelector: &pkg.NamespaceSelector{
				NameSelector: &pkg.NameSelector{
					MatchNames: []string{settings.ModuleNamespace},
				},
			},
			NameSelector: &pkg.NameSelector{
				MatchNames: managedTLSSecretNames,
			},
		},
	},
}

type tlsSecretSnapshot struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func Reconcile(_ context.Context, input *pkg.HookInput) error {
	for _, snapshot := range input.Snapshots.Get(tlsSecretsSnapshotName) {
		secret, err := readTLSSnapshot(snapshot)
		if err != nil {
			return err
		}
		if secret.Type == expectedKubernetesTLSRaw {
			continue
		}
		if !isManagedTLSSecret(secret.Name) {
			continue
		}
		input.PatchCollector.DeleteInBackground(secretAPIVersion, secretKind, settings.ModuleNamespace, secret.Name)
	}

	return nil
}

func readTLSSnapshot(snapshot pkg.Snapshot) (tlsSecretSnapshot, error) {
	var secret tlsSecretSnapshot
	if err := snapshot.UnmarshalTo(&secret); err != nil {
		return tlsSecretSnapshot{}, fmt.Errorf("unmarshal TLS secret snapshot: %w", err)
	}
	return secret, nil
}

func isManagedTLSSecret(name string) bool {
	for _, managedName := range managedTLSSecretNames {
		if name == managedName {
			return true
		}
	}
	return false
}
