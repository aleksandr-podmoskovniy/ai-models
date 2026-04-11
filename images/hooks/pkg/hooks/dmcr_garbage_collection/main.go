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

package dmcr_garbage_collection

import (
	"context"

	"github.com/deckhouse/module-sdk/pkg"
	"github.com/deckhouse/module-sdk/pkg/registry"
)

const (
	secretSnapshotName     = "dmcr-garbage-collection-secret"
	internalValuesPath     = "aiModels.internal.dmcr"
	requestLabelKey        = "ai-models.deckhouse.io/dmcr-gc-request"
	requestLabelValue      = "true"
	switchAnnotationKey    = "ai-models.deckhouse.io/dmcr-gc-switch"
	moduleNamespace        = "d8-ai-models"
	secretSnapshotJQFilter = `{
		"metadata": {
			"name": .metadata.name,
			"labels": .metadata.labels,
			"annotations": .metadata.annotations
		}
	}`
)

type partialSecret struct {
	Metadata partialSecretMetadata `json:"metadata"`
}

type partialSecretMetadata struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

var _ = registry.RegisterFunc(dmcrGarbageCollectionConfig, handleDMCRGarbageCollection)

var dmcrGarbageCollectionConfig = &pkg.HookConfig{
	OnBeforeHelm: &pkg.OrderedConfig{Order: 5},
	Kubernetes: []pkg.KubernetesConfig{
		{
			Name:       secretSnapshotName,
			APIVersion: "v1",
			Kind:       "Secret",
			JqFilter:   secretSnapshotJQFilter,
			NamespaceSelector: &pkg.NamespaceSelector{
				NameSelector: &pkg.NameSelector{
					MatchNames: []string{moduleNamespace},
				},
			},
		},
	},
}

func handleDMCRGarbageCollection(_ context.Context, input *pkg.HookInput) error {
	enabled := false
	for _, snapshot := range input.Snapshots.Get(secretSnapshotName) {
		var secret partialSecret
		if err := snapshot.UnmarshalTo(&secret); err != nil {
			return err
		}
		if secret.Metadata.Labels[requestLabelKey] != requestLabelValue {
			continue
		}
		if secret.Metadata.Annotations[switchAnnotationKey] == "" {
			continue
		}
		enabled = true
		break
	}
	input.Values.Set(internalValuesPath, map[string]bool{
		"garbageCollectionModeEnabled": enabled,
	})
	return nil
}
