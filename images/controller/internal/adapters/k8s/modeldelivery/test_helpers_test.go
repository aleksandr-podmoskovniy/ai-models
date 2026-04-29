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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const testDeliveryAuthKey = "test-delivery-auth-key"

func publishedArtifact() publication.PublishedArtifact {
	return publication.PublishedArtifact{
		Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
		URI:       "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/model@sha256:deadbeef",
		Digest:    "sha256:deadbeef",
		MediaType: "application/vnd.cncf.model.manifest.v1+json",
		SizeBytes: 42,
	}
}

func legacyRuntimeImagePullSecretNameForTest(t *testing.T, ownerUID types.UID) string {
	t.Helper()
	name, err := resourcenames.RuntimeImagePullSecretName(ownerUID)
	if err != nil {
		t.Fatalf("RuntimeImagePullSecretName() error = %v", err)
	}
	return name
}

func countVolumeByName(volumes []corev1.Volume, name string) int {
	count := 0
	for _, volume := range volumes {
		if volume.Name == name {
			count++
		}
	}
	return count
}

func podTemplateWithCacheMount(containerName, volumeName, mountPath string) *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  containerName,
				Image: "example.com/runtime:latest",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      volumeName,
					MountPath: mountPath,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
		},
	}
}

func podTemplateWithPVCMount(containerName, volumeName, claimName, mountPath string) *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  containerName,
				Image: "example.com/runtime:latest",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      volumeName,
					MountPath: mountPath,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: claimName,
					},
				},
			}},
		},
	}
}

func podTemplateWithNodeCacheVolume(containerName string) *corev1.PodTemplateSpec {
	template := podTemplateWithoutCacheMount(containerName)
	addNodeCacheVolume(template, DefaultManagedCacheName)
	return template
}

func addNodeCacheVolume(template *corev1.PodTemplateSpec, name string) {
	template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:           NodeCacheCSIDriverName,
				ReadOnly:         ptrBool(true),
				VolumeAttributes: map[string]string{"user.deckhouse.io/cache": "enabled"},
			},
		},
	})
}

func podTemplateWithoutCacheMount(containerName string) *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  containerName,
				Image: "example.com/runtime:latest",
			}},
		},
	}
}

func envByName(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}
