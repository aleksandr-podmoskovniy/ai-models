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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const dexCAValuesPath = "aiModels.internal.discoveredDexCA"

type dexCASecret struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

func applyDexCAFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %w", err)
	}

	cert := secret.Data["ca.crt"]
	if len(cert) == 0 {
		cert = secret.Data["tls.crt"]
	}

	return dexCASecret{
		Name: obj.GetName(),
		Data: cert,
	}, nil
}

func preferredDexCASecrets(globalHTTPSMode string) []string {
	if globalHTTPSMode == "CustomCertificate" {
		return []string{"ingress-tls-customcertificate", "ingress-tls"}
	}
	return []string{"ingress-tls", "ingress-tls-customcertificate"}
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "dexIngressSecret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"ingress-tls", "ingress-tls-customcertificate"},
			},
			FilterFunc: applyDexCAFilter,
		},
	},
}, discoverDexCA)

func discoverDexCA(_ context.Context, input *go_hook.HookInput) error {
	secrets := input.Snapshots.Get("dexIngressSecret")
	if len(secrets) == 0 {
		input.Values.Remove(dexCAValuesPath)
		return nil
	}

	secretsByName := make(map[string][]byte, len(secrets))
	for secret, err := range sdkobjectpatch.SnapshotIter[dexCASecret](secrets) {
		if err != nil {
			return fmt.Errorf("cannot iterate over dex ingress secret snapshot: %w", err)
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
