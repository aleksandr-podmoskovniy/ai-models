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

package sourceworker

import (
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
)

func buildPodSpec(
	options Options,
	sourcePlan publicationapp.SourceWorkerPlan,
	projectedAuthSecretName string,
	container corev1.Container,
) corev1.PodSpec {
	return corev1.PodSpec{
		RestartPolicy:      corev1.RestartPolicyNever,
		ServiceAccountName: options.ServiceAccountName,
		ImagePullSecrets:   imagePullSecrets(options.ImagePullSecretName),
		Volumes:            buildVolumes(options, sourcePlan, projectedAuthSecretName),
		Containers:         []corev1.Container{container},
	}
}

func buildContainer(
	request publicationports.Request,
	sourcePlan publicationapp.SourceWorkerPlan,
	artifactURI string,
	options Options,
	projectedAuthSecretName string,
) corev1.Container {
	return corev1.Container{
		Name:            "publish",
		Image:           options.Image,
		ImagePullPolicy: imagePullPolicyFor(options),
		Args:            append([]string{"publish-worker"}, buildArgs(request, sourcePlan, artifactURI, options)...),
		Env:             buildEnv(options, sourcePlan, projectedAuthSecretName),
		VolumeMounts:    buildVolumeMounts(options, sourcePlan),
		Resources:       options.Resources,
	}
}

func imagePullPolicyFor(options Options) corev1.PullPolicy {
	if options.ImagePullPolicy != "" {
		return options.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

func imagePullSecrets(secretName string) []corev1.LocalObjectReference {
	if strings.TrimSpace(secretName) == "" {
		return nil
	}
	return []corev1.LocalObjectReference{{Name: strings.TrimSpace(secretName)}}
}

func buildLabels(owner publicationports.Owner) map[string]string {
	return resourcenames.OwnerLabels("ai-models-publication", owner.Kind, owner.Name, owner.UID, owner.Namespace)
}

func buildEnv(
	options Options,
	plan publicationapp.SourceWorkerPlan,
	projectedAuthSecretName string,
) []corev1.EnvVar {
	env := ociregistry.Env(options.OCIInsecure, options.OCIRegistrySecretName, options.OCIRegistryCASecretName)
	env = append(env,
		corev1.EnvVar{Name: "LOG_FORMAT", Value: options.LogFormat},
		corev1.EnvVar{Name: "LOG_LEVEL", Value: options.LogLevel},
	)
	env = append(env, storageprojection.Env(options.ObjectStorage)...)
	if plan.HuggingFace != nil && plan.HuggingFace.AuthSecretRef != nil {
		env = append(env, corev1.EnvVar{
			Name: "HF_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: projectedAuthSecretName},
					Key:                  "token",
				},
			},
		})
	}
	return env
}

func buildVolumeMounts(options Options, _ publicationapp.SourceWorkerPlan) []corev1.VolumeMount {
	var extra []corev1.VolumeMount
	extra = storageprojection.VolumeMounts(options.ObjectStorage.CASecretName, extra...)
	return append(ociregistry.VolumeMounts(options.OCIRegistryCASecretName), extra...)
}

func buildVolumes(
	options Options,
	_ publicationapp.SourceWorkerPlan,
	_ string,
) []corev1.Volume {
	var extra []corev1.Volume
	extra = storageprojection.Volumes(options.ObjectStorage.CASecretName, extra...)
	return append(ociregistry.Volumes(options.OCIRegistryCASecretName), extra...)
}
