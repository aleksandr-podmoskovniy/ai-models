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

package nodecacheruntime

import (
	"strconv"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DesiredPod(spec RuntimeSpec) (*corev1.Pod, error) {
	name, err := resourcenames.NodeCacheRuntimePodName(spec.NodeName)
	if err != nil {
		return nil, err
	}
	claimName, err := resourcenames.NodeCacheRuntimePVCName(spec.NodeName)
	if err != nil {
		return nil, err
	}

	volumes := []corev1.Volume{{
		Name: cacheRootVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
			},
		},
	}}
	volumeMounts := []corev1.VolumeMount{{
		Name:      cacheRootVolumeName,
		MountPath: nodecache.RuntimeCacheRootPath,
	}}

	if strings.TrimSpace(spec.OCIRegistryCASecret) != "" {
		volumes = append(volumes, corev1.Volume{
			Name: registryCASecretVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: strings.TrimSpace(spec.OCIRegistryCASecret),
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      registryCASecretVolume,
			MountPath: registryCAPath,
			ReadOnly:  true,
		})
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				ManagedLabelKey: ManagedLabelValue,
			},
			Annotations: map[string]string{
				NodeNameAnnotationKey: spec.NodeName,
			},
		},
		Spec: corev1.PodSpec{
			NodeName:           spec.NodeName,
			RestartPolicy:      corev1.RestartPolicyAlways,
			ServiceAccountName: spec.ServiceAccountName,
			ImagePullSecrets:   imagePullSecrets(spec.ImagePullSecretName),
			Tolerations: []corev1.Toleration{{
				Operator: corev1.TolerationOpExists,
			}},
			Volumes: volumes,
			Containers: []corev1.Container{{
				Name:            DefaultContainerName,
				Image:           spec.RuntimeImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            []string{nodecache.RuntimeCommand},
				Env:             podEnv(spec),
				VolumeMounts:    volumeMounts,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    mustParseQuantity("50m"),
						corev1.ResourceMemory: mustParseQuantity("64Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    mustParseQuantity("200m"),
						corev1.ResourceMemory: mustParseQuantity("128Mi"),
					},
				},
			}},
		},
	}, nil
}

func podEnv(spec RuntimeSpec) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "LOG_FORMAT", Value: "json"},
		{Name: nodecache.RuntimeCacheRootEnv, Value: nodecache.RuntimeCacheRootPath},
		{Name: nodecache.RuntimeMaxSizeEnv, Value: spec.MaxTotalSize},
		{Name: nodecache.RuntimeMaxUnusedAgeEnv, Value: spec.MaxUnusedAge},
		{Name: nodecache.RuntimeScanIntervalEnv, Value: spec.ScanInterval},
		{Name: nodecacheintent.RuntimeNamespaceEnv, Value: spec.IntentNamespace},
		{Name: nodecacheintent.RuntimeNodeNameEnv, Value: spec.NodeName},
		{Name: "PUBLICATION_OCI_INSECURE", Value: strconv.FormatBool(spec.OCIInsecure)},
		{
			Name: "AI_MODELS_OCI_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: spec.OCIAuthSecretName},
					Key:                  "username",
				},
			},
		},
		{
			Name: "AI_MODELS_OCI_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: spec.OCIAuthSecretName},
					Key:                  "password",
				},
			},
		},
	}
	if strings.TrimSpace(spec.OCIRegistryCASecret) != "" {
		env = append(env, corev1.EnvVar{Name: "AI_MODELS_OCI_CA_FILE", Value: registryCAFilePath})
	}
	return env
}

func imagePullSecrets(secretName string) []corev1.LocalObjectReference {
	if strings.TrimSpace(secretName) == "" {
		return nil
	}
	return []corev1.LocalObjectReference{{Name: strings.TrimSpace(secretName)}}
}
