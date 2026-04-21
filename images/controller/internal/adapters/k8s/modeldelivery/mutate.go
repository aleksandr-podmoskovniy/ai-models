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

	corev1 "k8s.io/api/core/v1"
)

func applyRendered(template *corev1.PodTemplateSpec, rendered Rendered, digest string, deliveryMode DeliveryMode, deliveryReason DeliveryReason) error {
	if template == nil {
		return errors.New("runtime delivery pod template must not be nil")
	}

	template.Spec.InitContainers = upsertContainer(template.Spec.InitContainers, rendered.InitContainer)
	template.Spec.Containers = upsertRuntimeEnv(template.Spec.Containers, rendered.RuntimeEnv)
	template.Spec.Volumes = upsertVolumes(template.Spec.Volumes, rendered.Volumes)
	template.Spec.ImagePullSecrets = upsertImagePullSecrets(template.Spec.ImagePullSecrets, rendered.ImagePullSecrets)
	template.Annotations = upsertAnnotations(template.Annotations, map[string]string{
		ResolvedDigestAnnotation:         digest,
		ResolvedArtifactURIAnnotation:    rendered.ArtifactURI,
		ResolvedArtifactFamilyAnnotation: rendered.ArtifactFamily,
		ResolvedDeliveryModeAnnotation:   string(deliveryMode),
		ResolvedDeliveryReasonAnnotation: string(deliveryReason),
	})
	return nil
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

func upsertRuntimeEnv(containers []corev1.Container, desired []corev1.EnvVar) []corev1.Container {
	if len(desired) == 0 {
		return containers
	}
	for index := range containers {
		containers[index].Env = upsertEnv(containers[index].Env, desired)
	}
	return containers
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

func upsertAnnotations(existing map[string]string, desired map[string]string) map[string]string {
	if existing == nil {
		existing = make(map[string]string, len(desired))
	}
	for key, value := range desired {
		existing[key] = value
	}
	return existing
}
