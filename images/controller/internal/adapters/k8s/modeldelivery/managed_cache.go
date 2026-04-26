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
	"fmt"
	"strings"

	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
)

func ensureManagedCacheMount(template *corev1.PodTemplateSpec, options ServiceOptions, artifact publication.PublishedArtifact, family string) error {
	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	if !managed.Enabled {
		return nil
	}

	cacheMount, found, err := resolveCacheMount(template, options.Render.CacheMountPath)
	if err != nil {
		return err
	}
	if found && cacheMount.VolumeName != managed.VolumeName {
		return nil
	}

	template.Spec.Volumes, err = upsertManagedCacheVolume(template.Spec.Volumes, managed, artifact, family)
	if err != nil {
		return err
	}
	if err := ensureManagedNodeSelector(template, managed.NodeSelector); err != nil {
		return err
	}
	template.Spec.Containers = ensureManagedCacheVolumeMounts(template.Spec.Containers, managed.VolumeName, options.Render.CacheMountPath)
	template.Spec.InitContainers = ensureManagedCacheVolumeMounts(template.Spec.InitContainers, managed.VolumeName, options.Render.CacheMountPath)
	return nil
}

func HasManagedCacheTemplateState(template *corev1.PodTemplateSpec, options ServiceOptions) bool {
	if template == nil {
		return false
	}

	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	if hasVolumeByName(template.Spec.Volumes, managed.VolumeName) {
		return true
	}
	return containersMountVolume(template.Spec.Containers, managed.VolumeName) ||
		containersMountVolume(template.Spec.InitContainers, managed.VolumeName)
}

func RemoveManagedCacheTemplateState(template *corev1.PodTemplateSpec, options ServiceOptions) bool {
	if template == nil {
		return false
	}

	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	changed := false
	containers, removed := removeVolumeMountsByName(template.Spec.Containers, managed.VolumeName)
	if removed {
		template.Spec.Containers = containers
		changed = true
	}
	initContainers, removed := removeVolumeMountsByName(template.Spec.InitContainers, managed.VolumeName)
	if removed {
		template.Spec.InitContainers = initContainers
		changed = true
	}
	volumes, removed := removeVolumeByName(template.Spec.Volumes, managed.VolumeName)
	if removed {
		template.Spec.Volumes = volumes
		changed = true
	}

	return changed
}

func upsertManagedCacheVolume(volumes []corev1.Volume, options ManagedCacheOptions, artifact publication.PublishedArtifact, family string) ([]corev1.Volume, error) {
	desired, err := managedCacheVolume(options, artifact, family)
	if err != nil {
		return nil, err
	}
	for index := range volumes {
		if volumes[index].Name != desired.Name {
			continue
		}
		volumes[index] = desired
		return volumes, nil
	}
	return append(volumes, desired), nil
}

func managedCacheVolume(options ManagedCacheOptions, artifact publication.PublishedArtifact, family string) (corev1.Volume, error) {
	artifactURI := strings.TrimSpace(artifact.URI)
	digest := strings.TrimSpace(artifact.Digest)
	if artifactURI == "" || digest == "" {
		return corev1.Volume{}, fmt.Errorf("runtime delivery managed cache CSI volume requires artifact URI and digest")
	}

	attributes := map[string]string{
		nodeCacheCSIAttributeArtifactURI:    artifactURI,
		nodeCacheCSIAttributeArtifactDigest: digest,
	}
	if family = strings.TrimSpace(family); family != "" {
		attributes[nodeCacheCSIAttributeArtifactFamily] = family
	}
	return corev1.Volume{
		Name: options.VolumeName,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:           NodeCacheCSIDriverName,
				ReadOnly:         ptrBool(true),
				VolumeAttributes: attributes,
			},
		},
	}, nil
}

func ptrBool(value bool) *bool {
	return &value
}

func ensureManagedNodeSelector(template *corev1.PodTemplateSpec, selector map[string]string) error {
	if len(selector) == 0 {
		return nil
	}
	if template.Spec.NodeSelector == nil {
		template.Spec.NodeSelector = map[string]string{}
	}
	for key, value := range selector {
		existing, found := template.Spec.NodeSelector[key]
		if found && existing != value {
			return fmt.Errorf("runtime delivery managed node-cache selector conflicts on %q: workload has %q, node-cache requires %q", key, existing, value)
		}
		template.Spec.NodeSelector[key] = value
	}
	return nil
}

func ensureManagedCacheVolumeMounts(containers []corev1.Container, volumeName, mountPath string) []corev1.Container {
	for index := range containers {
		if containerMountsPath(containers[index], mountPath) {
			continue
		}
		containers[index].VolumeMounts = append(containers[index].VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		})
	}
	return containers
}

func removeVolumeMountsByName(containers []corev1.Container, name string) ([]corev1.Container, bool) {
	removed := false
	for index := range containers {
		filtered := containers[index].VolumeMounts[:0]
		for _, mount := range containers[index].VolumeMounts {
			if mount.Name == name {
				removed = true
				continue
			}
			filtered = append(filtered, mount)
		}
		containers[index].VolumeMounts = filtered
	}
	return containers, removed
}

func removeVolumeByName(volumes []corev1.Volume, name string) ([]corev1.Volume, bool) {
	removed := false
	filtered := volumes[:0]
	for _, volume := range volumes {
		if volume.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, volume)
	}
	return filtered, removed
}

func hasVolumeByName(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func containersMountVolume(containers []corev1.Container, name string) bool {
	for _, container := range containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == name {
				return true
			}
		}
	}
	return false
}

func containerMountsPath(container corev1.Container, mountPath string) bool {
	expected := normalizeMountPath(mountPath)
	for _, mount := range container.VolumeMounts {
		if normalizeMountPath(mount.MountPath) == expected {
			return true
		}
	}
	return false
}

func RemoveManagedRuntimeEnv(containers []corev1.Container) ([]corev1.Container, bool) {
	removed := false
	for index := range containers {
		filtered := containers[index].Env[:0]
		for _, env := range containers[index].Env {
			switch env.Name {
			case ModelPathEnv, ModelDigestEnv, ModelFamilyEnv:
				removed = true
				continue
			default:
				filtered = append(filtered, env)
			}
		}
		containers[index].Env = filtered
	}
	return containers, removed
}

func HasManagedRuntimeEnv(containers []corev1.Container) bool {
	for _, container := range containers {
		for _, env := range container.Env {
			switch env.Name {
			case ModelPathEnv, ModelDigestEnv, ModelFamilyEnv:
				return true
			}
		}
	}
	return false
}
