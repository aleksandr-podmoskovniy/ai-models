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
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func PruneManagedCacheTemplateState(template *corev1.PodTemplateSpec, volumeName string, keepNames []string) bool {
	if template == nil {
		return false
	}
	keep := make(map[string]struct{}, len(keepNames))
	for _, name := range keepNames {
		if strings.TrimSpace(name) != "" {
			keep[name] = struct{}{}
		}
	}
	changed := false
	containers, removed := pruneManagedCacheVolumeMounts(template.Spec.Containers, volumeName, keep)
	if removed {
		template.Spec.Containers = containers
		changed = true
	}
	initContainers, removed := pruneManagedCacheVolumeMounts(template.Spec.InitContainers, volumeName, keep)
	if removed {
		template.Spec.InitContainers = initContainers
		changed = true
	}
	volumes, removed := pruneManagedCacheVolumes(template.Spec.Volumes, volumeName, keep)
	if removed {
		template.Spec.Volumes = volumes
		changed = true
	}
	return changed
}

func pruneManagedCacheVolumeMounts(containers []corev1.Container, name string, keep map[string]struct{}) ([]corev1.Container, bool) {
	removed := false
	prefix := strings.TrimSpace(name) + "-"
	for index := range containers {
		filtered := containers[index].VolumeMounts[:0]
		for _, mount := range containers[index].VolumeMounts {
			_, keepMount := keep[mount.Name]
			if !keepMount && (mount.Name == name || strings.HasPrefix(mount.Name, prefix)) {
				removed = true
				continue
			}
			filtered = append(filtered, mount)
		}
		containers[index].VolumeMounts = filtered
	}
	return containers, removed
}

func pruneManagedCacheVolumes(volumes []corev1.Volume, name string, keep map[string]struct{}) ([]corev1.Volume, bool) {
	removed := false
	prefix := strings.TrimSpace(name) + "-"
	filtered := volumes[:0]
	for _, volume := range volumes {
		_, keepVolume := keep[volume.Name]
		if !keepVolume && (volume.Name == name || strings.HasPrefix(volume.Name, prefix)) {
			removed = true
			continue
		}
		filtered = append(filtered, volume)
	}
	return filtered, removed
}
