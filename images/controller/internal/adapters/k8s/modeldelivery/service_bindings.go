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

package modeldelivery

import (
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
)

func normalizeApplyBindings(request ApplyRequest) ([]ModelBinding, bool, error) {
	if len(request.Bindings) == 0 {
		binding := ModelBinding{
			Alias:          "model",
			Artifact:       request.Artifact,
			ArtifactFamily: request.ArtifactFamily,
		}
		if err := validateModelBinding(binding); err != nil {
			return nil, false, err
		}
		return []ModelBinding{binding}, false, nil
	}

	seen := make(map[string]struct{}, len(request.Bindings))
	bindings := make([]ModelBinding, 0, len(request.Bindings))
	for _, binding := range request.Bindings {
		if err := validateModelBinding(binding); err != nil {
			return nil, false, err
		}
		if _, found := seen[binding.Alias]; found {
			return nil, false, errors.New("runtime delivery model aliases must be unique")
		}
		seen[binding.Alias] = struct{}{}
		bindings = append(bindings, binding)
	}
	return bindings, true, nil
}

func validateModelBinding(binding ModelBinding) error {
	if err := nodecache.ValidateModelAlias(binding.Alias); err != nil {
		return err
	}
	if err := binding.Artifact.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(binding.Artifact.Digest) == "" {
		return errors.New("runtime delivery artifact digest must not be empty")
	}
	return nil
}

func inputBindings(bindings []ModelBinding, aliasContract bool) []BindingInput {
	if !aliasContract {
		return nil
	}
	result := make([]BindingInput, 0, len(bindings))
	for _, binding := range bindings {
		result = append(result, BindingInput{
			Alias:          binding.Alias,
			Artifact:       binding.Artifact,
			ArtifactFamily: binding.ArtifactFamily,
		})
	}
	return result
}

func ensureManagedCacheTemplate(
	template *corev1.PodTemplateSpec,
	options ServiceOptions,
	bindings []ModelBinding,
	aliasContract bool,
) error {
	if aliasContract {
		return ensureManagedAliasCacheTemplate(template, options, bindings)
	}
	return ensureManagedCacheMount(template, options, bindings[0].Artifact, bindings[0].ArtifactFamily)
}

func ensureManagedAliasCacheTemplate(
	template *corev1.PodTemplateSpec,
	options ServiceOptions,
	bindings []ModelBinding,
) error {
	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	if !managed.Enabled {
		return nil
	}
	for _, binding := range bindings {
		volumeName := managedModelVolumeName(managed.VolumeName, binding.Alias)
		var err error
		template.Spec.Volumes, err = stampManagedCacheVolume(template.Spec.Volumes, volumeName, binding.Artifact, binding.ArtifactFamily)
		if err != nil {
			return err
		}
		mountPath := NamedModelPath(options.Render, binding.Alias)
		template.Spec.Containers = ensureManagedCacheVolumeMounts(template.Spec.Containers, volumeName, mountPath)
		template.Spec.InitContainers = ensureManagedCacheVolumeMounts(template.Spec.InitContainers, volumeName, mountPath)
	}
	return nil
}

func detectApplyTopology(
	template *corev1.PodTemplateSpec,
	hints TopologyHints,
	mountPath string,
	managedVolumeName string,
	managedAliasDirect bool,
) (CacheTopology, error) {
	if managedAliasDirect {
		return CacheTopology{
			Kind:           CacheTopologyDirect,
			CacheMount:     CacheMount{VolumeName: managedVolumeName, MountPath: mountPath},
			DeliveryMode:   DeliveryModeSharedDirect,
			DeliveryReason: DeliveryReasonNodeSharedRuntimePlane,
		}, nil
	}
	return detectCacheTopology(template, hints, mountPath, managedVolumeName)
}
