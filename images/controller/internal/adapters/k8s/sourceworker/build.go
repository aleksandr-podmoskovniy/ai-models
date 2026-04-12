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
	"encoding/base64"
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	httpAuthVolumeName = "http-auth"
	httpAuthMountPath  = "/etc/ai-models/http-auth"
)

func Build(request publicationports.Request, options Options, projectedAuthSecretName string) (*corev1.Pod, error) {
	sourcePlan, err := sourcePlan(request)
	if err != nil {
		return nil, err
	}
	return buildWithPlan(request, sourcePlan, options, projectedAuthSecretName)
}

func buildWithPlan(
	request publicationports.Request,
	sourcePlan publicationapp.SourceWorkerPlan,
	options Options,
	projectedAuthSecretName string,
) (*corev1.Pod, error) {
	options = normalizeOptions(options)
	if err := validateOptions(sourcePlan, options); err != nil {
		return nil, err
	}
	if err := validateProjectedAuthSecretName(sourcePlan, projectedAuthSecretName); err != nil {
		return nil, err
	}

	name, err := resourcenames.SourceWorkerPodName(request.Owner.UID)
	if err != nil {
		return nil, err
	}
	artifactURI, err := publicationartifact.BuildOCIArtifactReference(options.OCIRepositoryPrefix, request.Identity, request.Owner.UID)
	if err != nil {
		return nil, err
	}

	container := corev1.Container{
		Name:            "publish",
		Image:           options.Image,
		ImagePullPolicy: imagePullPolicyFor(options),
		Args:            append([]string{"publish-worker"}, buildArgs(request, sourcePlan, artifactURI, options)...),
		Env:             buildEnv(options, sourcePlan, projectedAuthSecretName),
		VolumeMounts:    buildVolumeMounts(options, sourcePlan),
		Resources:       options.Resources,
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   options.Namespace,
			Labels:      buildLabels(request.Owner),
			Annotations: resourcenames.OwnerAnnotations(request.Owner.Kind, request.Owner.Name, request.Owner.Namespace),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: options.ServiceAccountName,
			ImagePullSecrets:   imagePullSecrets(options.ImagePullSecretName),
			Volumes:            buildVolumes(options, sourcePlan, projectedAuthSecretName),
			Containers:         []corev1.Container{container},
		},
	}, nil
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
	env = append(env, corev1.EnvVar{
		Name:  "TMPDIR",
		Value: workloadpod.WorkVolumeMountPath,
	})
	env = append(env, objectstorage.Env(options.ObjectStorage)...)
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

func buildVolumeMounts(options Options, plan publicationapp.SourceWorkerPlan) []corev1.VolumeMount {
	var extra []corev1.VolumeMount
	if plan.HTTP != nil && plan.HTTP.AuthSecretRef != nil {
		extra = append(extra, corev1.VolumeMount{
			Name:      httpAuthVolumeName,
			MountPath: httpAuthMountPath,
			ReadOnly:  true,
		})
	}
	extra = objectstorage.VolumeMounts(options.ObjectStorage.CASecretName, extra...)
	return workloadpod.VolumeMounts(options.RuntimeOptions, extra...)
}

func buildVolumes(
	options Options,
	plan publicationapp.SourceWorkerPlan,
	projectedAuthSecretName string,
) []corev1.Volume {
	var extra []corev1.Volume
	if plan.HTTP != nil && plan.HTTP.AuthSecretRef != nil {
		extra = append(extra, corev1.Volume{
			Name: httpAuthVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: projectedAuthSecretName,
				},
			},
		})
	}
	extra = objectstorage.Volumes(options.ObjectStorage.CASecretName, extra...)
	return workloadpod.Volumes(options.RuntimeOptions, extra...)
}

func buildArgs(
	request publicationports.Request,
	plan publicationapp.SourceWorkerPlan,
	artifactURI string,
	options Options,
) []string {
	args := []string{
		"--artifact-uri", artifactURI,
		"--source-type", string(plan.SourceType),
		"--snapshot-dir", workloadpod.WorkVolumeMountPath,
	}
	if strings.TrimSpace(string(plan.InputFormat)) != "" {
		args = append(args, "--input-format", string(plan.InputFormat))
	}
	if plan.Task != "" {
		args = append(args, "--task", plan.Task)
	}
	for _, engine := range plan.RuntimeEngines {
		args = append(args, "--runtime-engine", engine)
	}
	return append(args, sourceArgs(plan, request.Owner.UID, options.ObjectStorage.Bucket)...)
}

func sourceArgs(plan publicationapp.SourceWorkerPlan, ownerUID types.UID, rawBucket string) []string {
	if plan.HuggingFace != nil {
		return append(huggingFaceArgs(plan.HuggingFace), remoteRawStageArgs(ownerUID, rawBucket)...)
	}
	if plan.HTTP != nil {
		return append(httpArgs(plan.HTTP), remoteRawStageArgs(ownerUID, rawBucket)...)
	}
	if plan.Upload != nil {
		return uploadArgs(plan.Upload)
	}
	return nil
}

func huggingFaceArgs(source *publicationapp.HuggingFaceSourcePlan) []string {
	args := []string{"--hf-model-id", source.RepoID}
	if strings.TrimSpace(source.Revision) != "" {
		args = append(args, "--revision", source.Revision)
	}
	return args
}

func httpArgs(source *publicationapp.HTTPSourcePlan) []string {
	args := []string{"--http-url", source.URL}
	if len(source.CABundle) > 0 {
		args = append(args, "--http-ca-bundle-b64", base64.StdEncoding.EncodeToString(source.CABundle))
	}
	if source.AuthSecretRef != nil {
		args = append(args, "--http-auth-dir", httpAuthMountPath)
	}
	return args
}

func uploadArgs(source *publicationapp.UploadSourcePlan) []string {
	args := []string{
		"--upload-stage-bucket", source.Stage.Bucket,
		"--upload-stage-key", source.Stage.Key,
	}
	if strings.TrimSpace(source.Stage.FileName) != "" {
		args = append(args, "--upload-stage-file-name", source.Stage.FileName)
	}
	return args
}

func remoteRawStageArgs(ownerUID types.UID, rawBucket string) []string {
	if strings.TrimSpace(rawBucket) == "" {
		return nil
	}

	keyPrefix, err := resourcenames.UploadStagingObjectPrefix(ownerUID)
	if err != nil {
		return nil
	}

	return []string{
		"--raw-stage-bucket", rawBucket,
		"--raw-stage-key-prefix", keyPrefix + "/source-url",
	}
}

func validateProjectedAuthSecretName(
	plan publicationapp.SourceWorkerPlan,
	projectedAuthSecretName string,
) error {
	if !sourceAuthRequired(plan) {
		return nil
	}
	if strings.TrimSpace(projectedAuthSecretName) == "" {
		return errors.New("source worker projected auth secret name must not be empty when authSecretRef is set")
	}
	return nil
}

func sourceAuthRequired(plan publicationapp.SourceWorkerPlan) bool {
	return (plan.HuggingFace != nil && plan.HuggingFace.AuthSecretRef != nil) ||
		(plan.HTTP != nil && plan.HTTP.AuthSecretRef != nil)
}
