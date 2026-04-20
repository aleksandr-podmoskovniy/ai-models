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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func ensureManagedCacheMount(template *corev1.PodTemplateSpec, options ServiceOptions) error {
	managed := NormalizeManagedCacheOptions(options.ManagedCache)
	if !managed.Enabled {
		return nil
	}

	_, found, err := resolveCacheMount(template, options.Render.CacheMountPath)
	if err != nil || found {
		return err
	}

	template.Spec.Volumes, err = upsertManagedCacheVolume(template.Spec.Volumes, managed)
	if err != nil {
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

func upsertManagedCacheVolume(volumes []corev1.Volume, options ManagedCacheOptions) ([]corev1.Volume, error) {
	desired, err := managedCacheVolume(options)
	if err != nil {
		return nil, err
	}
	for index := range volumes {
		if volumes[index].Name != desired.Name {
			continue
		}
		if !equality.Semantic.DeepEqual(volumes[index], desired) {
			return nil, fmt.Errorf("runtime delivery managed cache volume %q already exists with different source", desired.Name)
		}
		return volumes, nil
	}
	return append(volumes, desired), nil
}

func managedCacheVolume(options ManagedCacheOptions) (corev1.Volume, error) {
	quantity, err := resource.ParseQuantity(strings.TrimSpace(options.VolumeSize))
	if err != nil {
		return corev1.Volume{}, fmt.Errorf("parse managed cache volume size: %w", err)
	}

	return corev1.Volume{
		Name: options.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Ephemeral: &corev1.EphemeralVolumeSource{
				VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"ai.deckhouse.io/managed-by": "ai-models",
							"ai.deckhouse.io/node-cache": "fallback",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: ptr.To(options.StorageClassName),
						VolumeMode:       ptr.To(corev1.PersistentVolumeFilesystem),
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: quantity,
							},
						},
					},
				},
			},
		},
	}, nil
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
