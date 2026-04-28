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
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

	volumes := []corev1.Volume{
		{
			Name: cacheRootVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: claimName,
				},
			},
		},
		hostPathVolume(csiPluginVolumeName, nodecache.CSIKubeletPluginDir, corev1.HostPathDirectoryOrCreate),
		hostPathVolume(csiRegistryVolumeName, nodecache.CSIRegistrationDirectory, corev1.HostPathDirectory),
		hostPathVolume(kubeletVolumeName, kubeletHostPath, corev1.HostPathDirectory),
		hostPathVolume(deviceVolumeName, deviceHostPath, corev1.HostPathDirectory),
	}
	volumeMounts := []corev1.VolumeMount{{
		Name:      cacheRootVolumeName,
		MountPath: nodecache.RuntimeCacheRootPath,
	}, {
		Name:      csiPluginVolumeName,
		MountPath: csiPluginMountPath,
	}, {
		Name:             kubeletVolumeName,
		MountPath:        kubeletHostPath,
		MountPropagation: ptr.To(corev1.MountPropagationBidirectional),
	}, {
		Name:      deviceVolumeName,
		MountPath: deviceHostPath,
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
			RestartPolicy:      corev1.RestartPolicyAlways,
			DNSPolicy:          corev1.DNSClusterFirst,
			SchedulerName:      corev1.DefaultSchedulerName,
			ServiceAccountName: spec.ServiceAccountName,
			ImagePullSecrets:   imagePullSecrets(spec.ImagePullSecretName),
			Affinity:           runtimeNodeAffinity(spec),
			Tolerations: []corev1.Toleration{{
				Operator: corev1.TolerationOpExists,
			}},
			Volumes: volumes,
			Containers: []corev1.Container{{
				Name:            DefaultContainerName,
				Image:           spec.RuntimeImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            []string{"--csi-endpoint=" + nodecache.CSIContainerSocketPath},
				Env:             podEnv(spec),
				VolumeMounts:    volumeMounts,
				SecurityContext: runtimeSecurityContext(),
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
			}, registrarContainer(spec)},
		},
	}, nil
}

func runtimeNodeAffinity(spec RuntimeSpec) *corev1.Affinity {
	hostname := strings.TrimSpace(spec.NodeHostname)
	if hostname == "" {
		hostname = strings.TrimSpace(spec.NodeName)
	}
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      corev1.LabelHostname,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{hostname},
					}},
				}},
			},
		},
	}
}

func registrarContainer(spec RuntimeSpec) corev1.Container {
	return corev1.Container{
		Name:            RegistrarContainerName,
		Image:           spec.CSIRegistrarImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"--v=5",
			"--csi-address=$(CSI_ENDPOINT)",
			"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
		},
		Env: []corev1.EnvVar{{
			Name:  "CSI_ENDPOINT",
			Value: nodecache.CSIContainerSocketPath,
		}, {
			Name:  "DRIVER_REG_SOCK_PATH",
			Value: nodecache.CSIKubeletSocketPath,
		}, {
			Name: "KUBE_NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
			},
		}},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      csiPluginVolumeName,
			MountPath: csiPluginMountPath,
		}, {
			Name:      csiRegistryVolumeName,
			MountPath: csiRegistryMountPath,
		}},
		SecurityContext: registrarSecurityContext(),
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    mustParseQuantity("12m"),
				corev1.ResourceMemory: mustParseQuantity("25Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    mustParseQuantity("25m"),
				corev1.ResourceMemory: mustParseQuantity("50Mi"),
			},
		},
	}
}

func hostPathVolume(name, path string, hostPathType corev1.HostPathType) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path,
				Type: &hostPathType,
			},
		},
	}
}

func runtimeSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Privileged:               ptr.To(true),
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsUser:                ptr.To[int64](0),
		RunAsNonRoot:             ptr.To(false),
		AllowPrivilegeEscalation: ptr.To(true),
		Capabilities: &corev1.Capabilities{
			Add: []corev1.Capability{"SYS_ADMIN"},
		},
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

func registrarSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsUser:                ptr.To[int64](0),
		RunAsNonRoot:             ptr.To(false),
		AllowPrivilegeEscalation: ptr.To(false),
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

func podEnv(spec RuntimeSpec) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "LOG_FORMAT", Value: "json"},
		{Name: nodecache.RuntimeCacheRootEnv, Value: nodecache.RuntimeCacheRootPath},
		{Name: nodecache.RuntimeMaxSizeEnv, Value: spec.MaxTotalSize},
		{Name: nodecache.RuntimeMaxUnusedAgeEnv, Value: spec.MaxUnusedAge},
		{Name: nodecache.RuntimeScanIntervalEnv, Value: spec.ScanInterval},
		{Name: RuntimeNodeNameEnv, Value: spec.NodeName},
		{
			Name: RuntimePodNameEnv,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: RuntimePodNamespaceEnv,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			},
		},
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
