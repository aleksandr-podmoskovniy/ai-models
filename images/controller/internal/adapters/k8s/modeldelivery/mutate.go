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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	corev1 "k8s.io/api/core/v1"
)

func applyRendered(template *corev1.PodTemplateSpec, rendered Rendered, digest string, deliveryMode DeliveryMode, deliveryReason DeliveryReason) error {
	if template == nil {
		return errors.New("runtime delivery pod template must not be nil")
	}

	RemoveSchedulingGate(template)
	switch {
	case len(rendered.InitContainers) > 0:
		template.Spec.InitContainers = upsertContainers(template.Spec.InitContainers, rendered.InitContainers)
		template.Spec.InitContainers = removeManagedInitContainersExcept(template.Spec.InitContainers, rendered.InitContainerName, rendered.InitContainerNames)
	default:
		template.Spec.InitContainers = removeManagedInitContainersExcept(template.Spec.InitContainers, rendered.InitContainerName, nil)
	}
	template.Spec.Containers = replaceManagedRuntimeEnv(template.Spec.Containers, rendered.RuntimeEnv)
	containers, err := upsertRuntimeVolumeMounts(template.Spec.Containers, rendered.RuntimeVolumeMounts)
	if err != nil {
		return err
	}
	template.Spec.Containers = containers
	template.Spec.Volumes = upsertVolumes(template.Spec.Volumes, rendered.Volumes)
	template.Spec.Volumes = removeUnreferencedVolumeByName(template.Spec.Volumes, template.Spec, ociregistry.CAVolumeName)
	template.Spec.ImagePullSecrets = upsertImagePullSecrets(template.Spec.ImagePullSecrets, rendered.ImagePullSecrets)
	template.Spec.ImagePullSecrets = removeImagePullSecretsByName(template.Spec.ImagePullSecrets, rendered.ImagePullSecretNamesPrune)
	template.Annotations = reconcileAnnotations(template.Annotations, map[string]string{
		ResolvedDigestAnnotation:         digest,
		ResolvedArtifactURIAnnotation:    rendered.ArtifactURI,
		ResolvedArtifactFamilyAnnotation: rendered.ArtifactFamily,
		ResolvedDeliveryModeAnnotation:   string(deliveryMode),
		ResolvedDeliveryReasonAnnotation: string(deliveryReason),
		ResolvedModelsAnnotation:         rendered.ResolvedModels,
	})
	return nil
}

func upsertContainers(existing []corev1.Container, desired []corev1.Container) []corev1.Container {
	for _, container := range desired {
		existing = upsertContainer(existing, container)
	}
	return existing
}

func upsertContainer(existing []corev1.Container, desired corev1.Container) []corev1.Container {
	for i := range existing {
		if existing[i].Name == desired.Name {
			existing[i] = desired
			return existing
		}
	}
	return append(existing, desired)
}

func replaceManagedRuntimeEnv(containers []corev1.Container, desired []corev1.EnvVar) []corev1.Container {
	for index := range containers {
		filtered := containers[index].Env[:0]
		for _, env := range containers[index].Env {
			if isManagedRuntimeEnv(env.Name) {
				continue
			}
			filtered = append(filtered, env)
		}
		containers[index].Env = upsertEnv(filtered, desired)
	}
	return containers
}

func upsertRuntimeVolumeMounts(containers []corev1.Container, desired []corev1.VolumeMount) ([]corev1.Container, error) {
	if len(desired) == 0 {
		return containers, nil
	}
	for index := range containers {
		mounts, err := upsertVolumeMounts(containers[index].VolumeMounts, desired)
		if err != nil {
			return nil, err
		}
		containers[index].VolumeMounts = mounts
	}
	return containers, nil
}

func upsertVolumeMounts(existing []corev1.VolumeMount, desired []corev1.VolumeMount) ([]corev1.VolumeMount, error) {
	for _, item := range desired {
		replaced := false
		for index := range existing {
			switch {
			case existing[index].Name == item.Name:
				existing[index] = item
				replaced = true
			case normalizeMountPath(existing[index].MountPath) == normalizeMountPath(item.MountPath):
				return nil, errors.New("runtime delivery volume mount path conflicts with existing workload mount")
			}
			if replaced {
				break
			}
		}
		if !replaced {
			existing = append(existing, item)
		}
	}
	return existing, nil
}

func upsertVolumes(existing []corev1.Volume, desired []corev1.Volume) []corev1.Volume {
	for _, item := range desired {
		replaced := false
		for i := range existing {
			if existing[i].Name == item.Name {
				existing[i] = item
				replaced = true
				break
			}
		}
		if !replaced {
			existing = append(existing, item)
		}
	}
	return existing
}

func removeManagedInitContainersExcept(existing []corev1.Container, baseName string, keep []string) []corev1.Container {
	if strings.TrimSpace(baseName) == "" {
		return existing
	}
	keepSet := make(map[string]struct{}, len(keep))
	for _, name := range keep {
		if strings.TrimSpace(name) != "" {
			keepSet[name] = struct{}{}
		}
	}
	prefix := strings.TrimSpace(baseName) + "-"
	filtered := existing[:0]
	for _, container := range existing {
		if container.Name == baseName || strings.HasPrefix(container.Name, prefix) {
			if _, found := keepSet[container.Name]; !found {
				continue
			}
		}
		filtered = append(filtered, container)
	}
	return filtered
}

func upsertEnv(existing []corev1.EnvVar, desired []corev1.EnvVar) []corev1.EnvVar {
	for _, item := range desired {
		replaced := false
		for index := range existing {
			if existing[index].Name == item.Name {
				existing[index] = item
				replaced = true
				break
			}
		}
		if !replaced {
			existing = append(existing, item)
		}
	}
	return existing
}

func upsertImagePullSecrets(existing []corev1.LocalObjectReference, desired []corev1.LocalObjectReference) []corev1.LocalObjectReference {
	for _, item := range desired {
		replaced := false
		for index := range existing {
			if existing[index].Name == item.Name {
				existing[index] = item
				replaced = true
				break
			}
		}
		if !replaced {
			existing = append(existing, item)
		}
	}
	return existing
}

func removeUnreferencedVolumeByName(volumes []corev1.Volume, spec corev1.PodSpec, name string) []corev1.Volume {
	if strings.TrimSpace(name) == "" || volumeNameIsMounted(spec, name) {
		return volumes
	}
	filtered := volumes[:0]
	for _, volume := range volumes {
		if volume.Name == name {
			continue
		}
		filtered = append(filtered, volume)
	}
	return filtered
}

func volumeNameIsMounted(spec corev1.PodSpec, name string) bool {
	return containersMountVolume(spec.Containers, name) || containersMountVolume(spec.InitContainers, name)
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

func removeImagePullSecretsByName(existing []corev1.LocalObjectReference, names []string) []corev1.LocalObjectReference {
	if len(existing) == 0 || len(names) == 0 {
		return existing
	}
	remove := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name != "" {
			remove[name] = struct{}{}
		}
	}
	filtered := existing[:0]
	for _, secret := range existing {
		if _, found := remove[secret.Name]; found {
			continue
		}
		filtered = append(filtered, secret)
	}
	return filtered
}

func reconcileAnnotations(existing map[string]string, desired map[string]string) map[string]string {
	if existing == nil {
		existing = make(map[string]string, len(desired))
	}
	for key, value := range desired {
		value = strings.TrimSpace(value)
		if value == "" {
			delete(existing, key)
			continue
		}
		existing[key] = value
	}
	if len(existing) == 0 {
		return nil
	}
	return existing
}
