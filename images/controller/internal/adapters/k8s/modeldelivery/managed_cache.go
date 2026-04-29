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

	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
)

func ensureManagedCacheMount(template *corev1.PodTemplateSpec, options ServiceOptions, artifact publication.PublishedArtifact, family string) error {
	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	if !managed.Enabled {
		return nil
	}

	volumeName := managed.VolumeName
	if cacheMount, found, err := resolveCacheMount(template, options.Render.CacheMountPath); err != nil {
		return err
	} else if found {
		if volume, volumeFound := findVolumeByName(template.Spec.Volumes, cacheMount.VolumeName); volumeFound && !isNodeCacheCSIVolume(volume) {
			return unsupportedCacheVolumeError(cacheMount, volume)
		}
		volumeName = cacheMount.VolumeName
	}
	var err error
	template.Spec.Volumes, err = stampManagedCacheVolume(template.Spec.Volumes, volumeName, artifact, family)
	if err != nil {
		return err
	}
	template.Spec.Containers = ensureManagedCacheVolumeMounts(template.Spec.Containers, volumeName, options.Render.CacheMountPath)
	template.Spec.InitContainers = ensureManagedCacheVolumeMounts(template.Spec.InitContainers, volumeName, options.Render.CacheMountPath)
	return nil
}

func HasManagedCacheTemplateState(template *corev1.PodTemplateSpec, options ServiceOptions) bool {
	if template == nil {
		return false
	}

	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	if hasManagedCacheVolume(template.Spec.Volumes, managed.VolumeName) {
		return true
	}
	return containersMountManagedVolume(template.Spec.Containers, managed.VolumeName) ||
		containersMountManagedVolume(template.Spec.InitContainers, managed.VolumeName)
}

func RemoveManagedCacheTemplateState(template *corev1.PodTemplateSpec, options ServiceOptions) bool {
	if template == nil {
		return false
	}

	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	return PruneManagedCacheTemplateState(template, managed.VolumeName, nil)
}

func stampManagedCacheVolume(volumes []corev1.Volume, volumeName string, artifact publication.PublishedArtifact, family string) ([]corev1.Volume, error) {
	volumeName = strings.TrimSpace(volumeName)
	for index := range volumes {
		if volumes[index].Name != volumeName {
			continue
		}
		if err := stampNodeCacheCSIVolume(&volumes[index], artifact, family); err != nil {
			return nil, err
		}
		return volumes, nil
	}
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   NodeCacheCSIDriverName,
				ReadOnly: ptrBool(true),
			},
		},
	}
	if err := stampNodeCacheCSIVolume(&volume, artifact, family); err != nil {
		return nil, err
	}
	return append(volumes, volume), nil
}

func stampNodeCacheCSIVolume(volume *corev1.Volume, artifact publication.PublishedArtifact, family string) error {
	if volume == nil {
		return errors.New("runtime delivery node-cache CSI volume must not be nil")
	}
	if volume.CSI == nil || strings.TrimSpace(volume.CSI.Driver) != NodeCacheCSIDriverName {
		return NewWorkloadContractError("runtime delivery volume %q must use node-cache CSI driver %q", volume.Name, NodeCacheCSIDriverName)
	}
	artifactURI := strings.TrimSpace(artifact.URI)
	digest := strings.TrimSpace(artifact.Digest)
	if artifactURI == "" || digest == "" {
		return fmt.Errorf("runtime delivery node-cache CSI volume %q requires artifact URI and digest", volume.Name)
	}

	attributes := copyVolumeAttributes(volume.CSI.VolumeAttributes)
	attributes[nodeCacheCSIAttributeArtifactURI] = artifactURI
	attributes[nodeCacheCSIAttributeArtifactDigest] = digest
	if family = strings.TrimSpace(family); family != "" {
		attributes[nodeCacheCSIAttributeArtifactFamily] = family
	} else {
		delete(attributes, nodeCacheCSIAttributeArtifactFamily)
	}
	volume.CSI.VolumeAttributes = attributes
	volume.CSI.ReadOnly = ptrBool(true)
	return nil
}

func copyVolumeAttributes(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	output := make(map[string]string, len(input)+3)
	for key, value := range input {
		output[key] = value
	}
	return output
}

func ptrBool(value bool) *bool {
	return &value
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

func hasManagedCacheVolume(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if managedCacheNameMatches(volume.Name, name) {
			return true
		}
	}
	return false
}

func containersMountManagedVolume(containers []corev1.Container, name string) bool {
	for _, container := range containers {
		for _, mount := range container.VolumeMounts {
			if managedCacheNameMatches(mount.Name, name) {
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
			if isManagedRuntimeEnv(env.Name) {
				removed = true
				continue
			}
			filtered = append(filtered, env)
		}
		containers[index].Env = filtered
	}
	return containers, removed
}

func HasManagedRuntimeEnv(containers []corev1.Container) bool {
	for _, container := range containers {
		for _, env := range container.Env {
			if isManagedRuntimeEnv(env.Name) {
				return true
			}
		}
	}
	return false
}

func RemoveManagedInitContainers(containers []corev1.Container, baseName string) ([]corev1.Container, bool) {
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		return containers, false
	}
	removed := false
	prefix := baseName + "-"
	filtered := containers[:0]
	for _, container := range containers {
		if container.Name == baseName || strings.HasPrefix(container.Name, prefix) {
			removed = true
			continue
		}
		filtered = append(filtered, container)
	}
	return filtered, removed
}

func HasManagedInitContainer(containers []corev1.Container, baseName string) bool {
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		return false
	}
	prefix := baseName + "-"
	for _, container := range containers {
		if container.Name == baseName || strings.HasPrefix(container.Name, prefix) {
			return true
		}
	}
	return false
}

func isManagedRuntimeEnv(name string) bool {
	switch name {
	case ModelPathEnv, ModelDigestEnv, ModelFamilyEnv, ModelsDirEnv, ModelsEnv:
		return true
	default:
		return strings.HasPrefix(name, "AI_MODELS_MODEL_") &&
			(strings.HasSuffix(name, "_PATH") ||
				strings.HasSuffix(name, "_DIGEST") ||
				strings.HasSuffix(name, "_FAMILY"))
	}
}
