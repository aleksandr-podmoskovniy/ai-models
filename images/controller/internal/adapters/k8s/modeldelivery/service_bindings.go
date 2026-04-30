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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

func normalizeApplyBindings(request ApplyRequest) ([]ModelBinding, bool, error) {
	if len(request.Bindings) == 0 {
		return nil, false, errors.New("runtime delivery requires at least one model binding")
	}

	seen := make(map[string]struct{}, len(request.Bindings))
	bindings := make([]ModelBinding, 0, len(request.Bindings))
	for _, binding := range request.Bindings {
		if err := validateModelBinding(binding); err != nil {
			return nil, false, err
		}
		if _, found := seen[binding.Name]; found {
			return nil, false, errors.New("runtime delivery model names must be unique")
		}
		seen[binding.Name] = struct{}{}
		bindings = append(bindings, binding)
	}
	return bindings, true, nil
}

func validateModelBinding(binding ModelBinding) error {
	if err := validateModelName(binding.Name); err != nil {
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

func validateModelName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("runtime delivery model name must not be empty")
	}
	if problems := validation.IsDNS1123Subdomain(name); len(problems) > 0 {
		return fmt.Errorf("runtime delivery model name must be a valid Kubernetes object name: %s", strings.Join(problems, "; "))
	}
	return nil
}

func inputBindings(bindings []ModelBinding) []BindingInput {
	result := make([]BindingInput, 0, len(bindings))
	for _, binding := range bindings {
		result = append(result, BindingInput{
			Name:           binding.Name,
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
) error {
	return ensureManagedModelCacheTemplate(template, options, bindings)
}

func ensureManagedModelCacheTemplate(
	template *corev1.PodTemplateSpec,
	options ServiceOptions,
	bindings []ModelBinding,
) error {
	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	if !managed.Enabled {
		return nil
	}
	if cacheMount, found, err := resolveCacheMount(template, options.Render.CacheMountPath); err != nil {
		return err
	} else if found {
		if volume, volumeFound := findVolumeByName(template.Spec.Volumes, cacheMount.VolumeName); volumeFound && !isNodeCacheCSIVolume(volume) {
			return unsupportedCacheVolumeError(cacheMount, volume)
		}
	}
	for _, binding := range bindings {
		volumeName := managedModelVolumeName(managed.VolumeName, binding.Name)
		var err error
		template.Spec.Volumes, err = stampManagedCacheVolume(template.Spec.Volumes, volumeName, binding.Artifact, binding.ArtifactFamily)
		if err != nil {
			return err
		}
		mountPath := NamedModelPath(options.Render, binding.Name)
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
	managedDirect bool,
	sharedPVCClaimName string,
	sharedPVCVolumeName string,
) (CacheTopology, error) {
	if managedDirect {
		return CacheTopology{
			Kind:           CacheTopologyDirect,
			CacheMount:     CacheMount{VolumeName: managedVolumeName, MountPath: mountPath},
			DeliveryMode:   DeliveryModeSharedDirect,
			DeliveryReason: DeliveryReasonNodeSharedRuntimePlane,
		}, nil
	}
	if strings.TrimSpace(sharedPVCClaimName) != "" {
		if cacheMount, found, err := resolveCacheMount(template, mountPath); err != nil {
			return CacheTopology{}, err
		} else if found {
			return CacheTopology{}, NewWorkloadContractError("runtime delivery does not support explicit cache volume %q when SharedPVC is controller-owned", cacheMount.VolumeName)
		}
		return CacheTopology{
			Kind:           CacheTopologySharedPVC,
			CacheMount:     CacheMount{VolumeName: sharedPVCVolumeName, MountPath: mountPath},
			ClaimName:      sharedPVCClaimName,
			DeliveryMode:   DeliveryModeSharedPVC,
			DeliveryReason: DeliveryReasonRWXSharedVolume,
		}, nil
	}
	if _, found, err := resolveCacheMount(template, mountPath); err != nil {
		return CacheTopology{}, err
	} else if !found {
		return CacheTopology{}, NewWorkloadBlockedError(
			DeliveryGateReasonSharedPVCStorageClassMissing,
			"runtime delivery requires nodeCache.enabled=true or sharedPVC.storageClassName with an RWX StorageClass",
		)
	}
	return detectCacheTopology(template, hints, mountPath, managedVolumeName)
}
