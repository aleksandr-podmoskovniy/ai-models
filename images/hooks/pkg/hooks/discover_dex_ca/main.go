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

package discover_dex_ca

import (
	"context"
	"fmt"

	"github.com/deckhouse/module-sdk/pkg"
	"github.com/deckhouse/module-sdk/pkg/registry"
)

const (
	dexIngressSecretSnapshot = "dex-ingress-secret"
	dexCAValuesPath          = "aiModels.internal.discoveredDexCA"
	dexIngressSecretFilter   = `{
		"name": .metadata.name,
		"data": (.data."ca.crt" // .data."tls.crt")
	}`
)

type dexCASecret struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

var _ = registry.RegisterFunc(discoverDexCAConfig, discoverDexCA)

var discoverDexCAConfig = &pkg.HookConfig{
	OnBeforeHelm: &pkg.OrderedConfig{Order: 15},
	Kubernetes: []pkg.KubernetesConfig{
		{
			Name:       dexIngressSecretSnapshot,
			APIVersion: "v1",
			Kind:       "Secret",
			JqFilter:   dexIngressSecretFilter,
			NamespaceSelector: &pkg.NamespaceSelector{
				NameSelector: &pkg.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &pkg.NameSelector{
				MatchNames: []string{"ingress-tls", "ingress-tls-customcertificate"},
			},
		},
	},
}

func preferredDexCASecrets(globalHTTPSMode string) []string {
	if globalHTTPSMode == "CustomCertificate" {
		return []string{"ingress-tls-customcertificate", "ingress-tls"}
	}

	return []string{"ingress-tls", "ingress-tls-customcertificate"}
}

func discoverDexCA(_ context.Context, input *pkg.HookInput) error {
	snapshots := input.Snapshots.Get(dexIngressSecretSnapshot)
	if len(snapshots) == 0 {
		input.Values.Remove(dexCAValuesPath)
		return nil
	}

	secretsByName := make(map[string][]byte, len(snapshots))
	for _, snapshot := range snapshots {
		var secret dexCASecret
		if err := snapshot.UnmarshalTo(&secret); err != nil {
			return fmt.Errorf("unmarshal dex ingress secret snapshot: %w", err)
		}
		if len(secret.Data) == 0 {
			continue
		}
		secretsByName[secret.Name] = secret.Data
	}

	globalHTTPSMode := input.Values.Get("global.modules.https.mode").String()
	for _, secretName := range preferredDexCASecrets(globalHTTPSMode) {
		if cert, ok := secretsByName[secretName]; ok && len(cert) > 0 {
			input.Values.Set(dexCAValuesPath, string(cert))
			return nil
		}
	}

	input.Values.Remove(dexCAValuesPath)
	return nil
}
