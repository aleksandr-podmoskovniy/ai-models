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

package root_ca_discovery

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"

	"github.com/deckhouse/module-sdk/pkg"
	"github.com/deckhouse/module-sdk/pkg/registry"

	"hooks/pkg/settings"
)

const (
	rootCASecretName      = "ai-models-ca"
	rootCASnapshotName    = "ai-models-root-ca"
	rootCAValuesPath      = "aiModels.internal.rootCA"
	rootCASecretJQFilter  = `{"crt": .data."tls.crt", "key": .data."tls.key"}`
	moduleHookQueuePrefix = "modules"
)

type caSecret struct {
	Crt []byte `json:"crt"`
	Key []byte `json:"key"`
}

var _ = registry.RegisterFunc(config, Reconcile)

var config = &pkg.HookConfig{
	OnBeforeHelm: &pkg.OrderedConfig{Order: 1},
	Kubernetes: []pkg.KubernetesConfig{
		{
			Name:       rootCASnapshotName,
			APIVersion: "v1",
			Kind:       "Secret",
			JqFilter:   rootCASecretJQFilter,
			NameSelector: &pkg.NameSelector{
				MatchNames: []string{rootCASecretName},
			},
			NamespaceSelector: &pkg.NamespaceSelector{
				NameSelector: &pkg.NameSelector{
					MatchNames: []string{settings.ModuleNamespace},
				},
			},
			ExecuteHookOnSynchronization: ptr.To(false),
		},
	},
	Queue: fmt.Sprintf("%s/%s", moduleHookQueuePrefix, settings.ModuleName),
}

func Reconcile(_ context.Context, input *pkg.HookInput) error {
	snapshots := input.Snapshots.Get(rootCASnapshotName)
	if len(snapshots) == 0 {
		return nil
	}

	var rootCA caSecret
	if err := snapshots[0].UnmarshalTo(&rootCA); err != nil {
		return fmt.Errorf("unmarshal root CA secret: %w", err)
	}

	input.Values.Set(rootCAValuesPath, rootCA)
	return nil
}
