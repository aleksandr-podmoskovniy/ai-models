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

package workloadpod

import (
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	corev1 "k8s.io/api/core/v1"
)

const (
	WorkVolumeName      = "work"
	WorkVolumeMountPath = "/var/lib/ai-models/work"
)

func VolumeMounts(options RuntimeOptions, extra ...corev1.VolumeMount) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{{
		Name:      WorkVolumeName,
		MountPath: WorkVolumeMountPath,
	}}
	mounts = append(mounts, ociregistry.VolumeMounts(options.OCIRegistryCASecretName)...)
	return append(mounts, extra...)
}

func Volumes(options RuntimeOptions, extra ...corev1.Volume) []corev1.Volume {
	volumes := []corev1.Volume{{
		Name:         WorkVolumeName,
		VolumeSource: workVolumeSource(options.WorkVolume),
	}}
	volumes = append(volumes, ociregistry.Volumes(options.OCIRegistryCASecretName)...)
	return append(volumes, extra...)
}

func workVolumeSource(options WorkVolumeOptions) corev1.VolumeSource {
	switch options.Type {
	case WorkVolumeTypePersistentVolumeClaim:
		return corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: options.PersistentVolumeClaimName,
			},
		}
	default:
		sizeLimit := options.EmptyDirSizeLimit.DeepCopy()
		return corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				SizeLimit: &sizeLimit,
			},
		}
	}
}
