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

package objectstorage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
)

const (
	caVolumeName = "artifacts-ca"
	caMountPath  = "/etc/ai-models/artifacts-ca"
	caFilePath   = caMountPath + "/ca.crt"
)

type Options struct {
	Bucket                string
	EndpointURL           string
	Region                string
	UsePathStyle          bool
	Insecure              bool
	CredentialsSecretName string
	CASecretName          string
}

func ValidateOptions(component string, options Options) error {
	component = strings.TrimSpace(component)
	if component == "" {
		return errors.New("object storage component name must not be empty")
	}

	switch {
	case strings.TrimSpace(options.Bucket) == "":
		return fmt.Errorf("%s object storage bucket must not be empty", component)
	case strings.TrimSpace(options.EndpointURL) == "":
		return fmt.Errorf("%s object storage endpoint URL must not be empty", component)
	case strings.TrimSpace(options.Region) == "":
		return fmt.Errorf("%s object storage region must not be empty", component)
	case strings.TrimSpace(options.CredentialsSecretName) == "":
		return fmt.Errorf("%s object storage credentials secret name must not be empty", component)
	default:
		return nil
	}
}

func Env(options Options) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "AI_MODELS_S3_BUCKET", Value: strings.TrimSpace(options.Bucket)},
		{Name: "AI_MODELS_S3_ENDPOINT_URL", Value: strings.TrimSpace(options.EndpointURL)},
		{Name: "AI_MODELS_S3_REGION", Value: strings.TrimSpace(options.Region)},
		{Name: "AI_MODELS_S3_USE_PATH_STYLE", Value: resourcenames.BoolString(options.UsePathStyle)},
		{Name: "AI_MODELS_S3_IGNORE_TLS", Value: resourcenames.BoolString(options.Insecure)},
		{Name: "AWS_REGION", Value: strings.TrimSpace(options.Region)},
		{Name: "AWS_DEFAULT_REGION", Value: strings.TrimSpace(options.Region)},
		{Name: "AWS_EC2_METADATA_DISABLED", Value: "true"},
		{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: options.CredentialsSecretName},
					Key:                  "accessKey",
				},
			},
		},
		{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: options.CredentialsSecretName},
					Key:                  "secretKey",
				},
			},
		},
	}
	if strings.TrimSpace(options.CASecretName) != "" {
		env = append(env, corev1.EnvVar{
			Name:  "AI_MODELS_S3_CA_FILE",
			Value: caFilePath,
		})
	}
	return env
}

func VolumeMounts(caSecretName string, extra ...corev1.VolumeMount) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, len(extra)+1)
	if strings.TrimSpace(caSecretName) != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      caVolumeName,
			MountPath: caMountPath,
			ReadOnly:  true,
		})
	}
	return append(mounts, extra...)
}

func Volumes(caSecretName string, extra ...corev1.Volume) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(extra)+1)
	if strings.TrimSpace(caSecretName) != "" {
		volumes = append(volumes, corev1.Volume{
			Name: caVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: caSecretName,
				},
			},
		})
	}
	return append(volumes, extra...)
}
