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
	keep := managedCacheKeepSet(keepNames)
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
	for index := range containers {
		filtered := containers[index].VolumeMounts[:0]
		for _, mount := range containers[index].VolumeMounts {
			if managedCacheNamePruned(mount.Name, name, keep) {
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
	filtered := volumes[:0]
	for _, volume := range volumes {
		if managedCacheNamePruned(volume.Name, name, keep) {
			removed = true
			continue
		}
		filtered = append(filtered, volume)
	}
	return filtered, removed
}

func managedCacheKeepSet(names []string) map[string]struct{} {
	keep := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			keep[name] = struct{}{}
		}
	}
	return keep
}

func managedCacheNamePruned(value, baseName string, keep map[string]struct{}) bool {
	if _, found := keep[value]; found {
		return false
	}
	return managedCacheNameMatches(value, baseName)
}

func managedCacheNameMatches(value, baseName string) bool {
	baseName = strings.TrimSpace(baseName)
	return value == baseName || strings.HasPrefix(value, baseName+"-")
}
