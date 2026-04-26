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

package workloaddelivery

import (
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func podTemplateAndHints(object client.Object) (*corev1.PodTemplateSpec, modeldelivery.TopologyHints, error) {
	switch typed := object.(type) {
	case *appsv1.Deployment:
		return &typed.Spec.Template, modeldelivery.HintsForDeployment(typed), nil
	case *appsv1.StatefulSet:
		return &typed.Spec.Template, modeldelivery.HintsForStatefulSet(typed), nil
	case *appsv1.DaemonSet:
		return &typed.Spec.Template, modeldelivery.HintsForDaemonSet(typed), nil
	case *batchv1.CronJob:
		return &typed.Spec.JobTemplate.Spec.Template, modeldelivery.HintsForCronJob(typed), nil
	default:
		return nil, modeldelivery.TopologyHints{}, fmt.Errorf("unsupported workload delivery object type %T", object)
	}
}

func removeManagedTemplateState(template *corev1.PodTemplateSpec, options modeldelivery.ServiceOptions) bool {
	if template == nil {
		return false
	}
	options = modeldelivery.NormalizeServiceOptions(options)

	changed := false
	if modeldelivery.RemoveSchedulingGate(template) {
		changed = true
	}
	initContainers, removedInit := modeldelivery.RemoveManagedInitContainers(template.Spec.InitContainers, options.Render.InitContainerName)
	if removedInit {
		template.Spec.InitContainers = initContainers
		changed = true
	}
	containers, removedRuntimeEnv := modeldelivery.RemoveManagedRuntimeEnv(template.Spec.Containers)
	if removedRuntimeEnv {
		template.Spec.Containers = containers
		changed = true
	}

	for _, key := range []string{
		modeldelivery.ResolvedDigestAnnotation,
		modeldelivery.ResolvedArtifactURIAnnotation,
		modeldelivery.ResolvedArtifactFamilyAnnotation,
		modeldelivery.ResolvedDeliveryModeAnnotation,
		modeldelivery.ResolvedDeliveryReasonAnnotation,
		modeldelivery.ResolvedModelsAnnotation,
	} {
		var removed bool
		template.Annotations, removed = removeAnnotation(template.Annotations, key)
		if removed {
			changed = true
		}
	}

	if !volumeMounted(template, ociregistry.CAVolumeName) {
		volumes, removedVolume := removeVolumeByName(template.Spec.Volumes, ociregistry.CAVolumeName)
		if removedVolume {
			template.Spec.Volumes = volumes
			changed = true
		}
	}

	if modeldelivery.RemoveManagedCacheTemplateState(template, options) {
		changed = true
	}

	return changed
}

func hasManagedTemplateState(template *corev1.PodTemplateSpec, options modeldelivery.ServiceOptions) bool {
	if template == nil {
		return false
	}
	options = modeldelivery.NormalizeServiceOptions(options)

	if strings.TrimSpace(template.Annotations[modeldelivery.ResolvedDigestAnnotation]) != "" ||
		strings.TrimSpace(template.Annotations[modeldelivery.ResolvedArtifactURIAnnotation]) != "" ||
		strings.TrimSpace(template.Annotations[modeldelivery.ResolvedArtifactFamilyAnnotation]) != "" ||
		strings.TrimSpace(template.Annotations[modeldelivery.ResolvedDeliveryModeAnnotation]) != "" ||
		strings.TrimSpace(template.Annotations[modeldelivery.ResolvedDeliveryReasonAnnotation]) != "" ||
		strings.TrimSpace(template.Annotations[modeldelivery.ResolvedModelsAnnotation]) != "" {
		return true
	}
	if modeldelivery.HasManagedRuntimeEnv(template.Spec.Containers) {
		return true
	}
	if modeldelivery.HasSchedulingGate(template) {
		return true
	}
	if modeldelivery.HasManagedInitContainer(template.Spec.InitContainers, options.Render.InitContainerName) {
		return true
	}
	for _, volume := range template.Spec.Volumes {
		if volume.Name == ociregistry.CAVolumeName {
			return true
		}
	}
	return modeldelivery.HasManagedCacheTemplateState(template, options)
}

func removeAnnotation(annotations map[string]string, key string) (map[string]string, bool) {
	if len(annotations) == 0 {
		return annotations, false
	}
	if _, found := annotations[key]; !found {
		return annotations, false
	}
	delete(annotations, key)
	if len(annotations) == 0 {
		return nil, true
	}
	return annotations, true
}

func volumeMounted(template *corev1.PodTemplateSpec, volumeName string) bool {
	for _, container := range template.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volumeName {
				return true
			}
		}
	}
	for _, container := range template.Spec.InitContainers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volumeName {
				return true
			}
		}
	}
	return false
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

func removeImagePullSecretByName(
	secrets []corev1.LocalObjectReference,
	name string,
) ([]corev1.LocalObjectReference, bool) {
	removed := false
	filtered := secrets[:0]
	for _, secret := range secrets {
		if secret.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, secret)
	}
	return filtered, removed
}
