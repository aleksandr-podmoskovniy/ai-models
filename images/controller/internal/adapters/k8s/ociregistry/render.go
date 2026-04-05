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

package ociregistry

import (
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
)

const (
	CAVolumeName = "registry-ca"
	CAFilePath   = "/etc/ai-models/registry-ca/ca.crt"
)

func Env(insecure bool, secretName, caSecretName string) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "AI_MODELS_OCI_INSECURE", Value: resourcenames.BoolString(insecure)},
		{
			Name: "AI_MODELS_OCI_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "username",
				},
			},
		},
		{
			Name: "AI_MODELS_OCI_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "password",
				},
			},
		},
	}

	if strings.TrimSpace(caSecretName) != "" {
		env = append(env, corev1.EnvVar{Name: "AI_MODELS_OCI_CA_FILE", Value: CAFilePath})
	}

	return env
}

func VolumeMounts(caSecretName string) []corev1.VolumeMount {
	if strings.TrimSpace(caSecretName) == "" {
		return nil
	}

	return []corev1.VolumeMount{{
		Name:      CAVolumeName,
		MountPath: "/etc/ai-models/registry-ca",
		ReadOnly:  true,
	}}
}

func Volumes(caSecretName string) []corev1.Volume {
	if strings.TrimSpace(caSecretName) == "" {
		return nil
	}

	return []corev1.Volume{{
		Name: CAVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: caSecretName,
			},
		},
	}}
}
