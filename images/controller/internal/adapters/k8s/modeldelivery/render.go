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
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
)

type Input struct {
	Artifact                   publication.PublishedArtifact
	ArtifactFamily             string
	RegistryAccess             ociregistry.Projection
	RuntimeImagePullSecretName string
	CacheMount                 CacheMount
	TopologyKind               CacheTopologyKind
	Coordination               Coordination
}

type Rendered struct {
	InitContainer             corev1.Container
	HasInitContainer          bool
	InitContainerName         string
	RuntimeEnv                []corev1.EnvVar
	Volumes                   []corev1.Volume
	ImagePullSecrets          []corev1.LocalObjectReference
	ImagePullSecretNamesPrune []string
	ModelPath                 string
	ArtifactURI               string
	ArtifactFamily            string
}

func Render(input Input, options Options) (Rendered, error) {
	options = NormalizeOptions(options)
	if err := ValidateOptions(options); err != nil {
		return Rendered{}, err
	}
	if err := input.Artifact.Validate(); err != nil {
		return Rendered{}, err
	}
	if strings.TrimSpace(input.Artifact.Digest) == "" {
		return Rendered{}, errors.New("runtime delivery artifact digest must not be empty")
	}
	if input.TopologyKind != CacheTopologyDirect && strings.TrimSpace(input.RegistryAccess.AuthSecretName) == "" {
		return Rendered{}, errors.New("runtime delivery registry auth projection must not be empty")
	}
	if strings.TrimSpace(input.CacheMount.VolumeName) == "" {
		return Rendered{}, errors.New("runtime delivery cache volume name must not be empty")
	}
	if normalizeMountPath(input.CacheMount.MountPath) != normalizeMountPath(options.CacheMountPath) {
		return Rendered{}, errors.New("runtime delivery cache mount contract is inconsistent")
	}

	initMounts := append([]corev1.VolumeMount{{
		Name:      input.CacheMount.VolumeName,
		MountPath: options.CacheMountPath,
	}}, ociregistry.VolumeMounts(input.RegistryAccess.CASecretName)...)

	modelPath := ModelPath(options)
	if input.TopologyKind == CacheTopologySharedPVC {
		modelPath = nodecache.SharedArtifactModelPath(options.CacheMountPath, input.Artifact.Digest)
	}
	if input.TopologyKind == CacheTopologyDirect {
		return Rendered{
			HasInitContainer:          false,
			InitContainerName:         options.InitContainerName,
			RuntimeEnv:                buildRuntimeEnv(input, options, modelPath),
			ImagePullSecretNamesPrune: buildImagePullSecretNamesPrune(input.RuntimeImagePullSecretName),
			ModelPath:                 modelPath,
			ArtifactURI:               strings.TrimSpace(input.Artifact.URI),
			ArtifactFamily:            strings.TrimSpace(input.ArtifactFamily),
		}, nil
	}

	return Rendered{
		HasInitContainer:  true,
		InitContainerName: options.InitContainerName,
		InitContainer: corev1.Container{
			Name:            options.InitContainerName,
			Image:           options.RuntimeImage,
			ImagePullPolicy: options.ImagePullPolicy,
			Args:            []string{"materialize-artifact"},
			Env:             buildInitEnv(input, options),
			VolumeMounts:    initMounts,
		},
		RuntimeEnv:       buildRuntimeEnv(input, options, modelPath),
		Volumes:          ociregistry.Volumes(input.RegistryAccess.CASecretName),
		ImagePullSecrets: buildImagePullSecrets(input.RuntimeImagePullSecretName),
		ModelPath:        modelPath,
		ArtifactURI:      strings.TrimSpace(input.Artifact.URI),
		ArtifactFamily:   strings.TrimSpace(input.ArtifactFamily),
	}, nil
}

func buildImagePullSecrets(secretName string) []corev1.LocalObjectReference {
	if strings.TrimSpace(secretName) == "" {
		return nil
	}
	return []corev1.LocalObjectReference{{Name: strings.TrimSpace(secretName)}}
}

func buildImagePullSecretNamesPrune(secretName string) []string {
	if strings.TrimSpace(secretName) == "" {
		return nil
	}
	return []string{strings.TrimSpace(secretName)}
}

func buildInitEnv(input Input, options Options) []corev1.EnvVar {
	env := ociregistry.Env(options.OCIInsecure, input.RegistryAccess.AuthSecretName, input.RegistryAccess.CASecretName)
	env = append(env,
		corev1.EnvVar{Name: LogFormatEnv, Value: options.LogFormat},
		corev1.EnvVar{Name: LogLevelEnv, Value: options.LogLevel},
		corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_ARTIFACT_URI", Value: input.Artifact.URI},
		corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_ARTIFACT_DIGEST", Value: input.Artifact.Digest},
		corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_CACHE_ROOT", Value: options.CacheMountPath},
	)
	if strings.TrimSpace(input.Coordination.Mode) == CoordinationModeShared {
		env = append(env,
			corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_COORDINATION_MODE", Value: input.Coordination.Mode},
			corev1.EnvVar{
				Name: "AI_MODELS_MATERIALIZE_COORDINATION_HOLDER_ID",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
				},
			},
		)
	}
	if input.TopologyKind == CacheTopologySharedPVC {
		env = append(env, corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_SHARED_STORE", Value: "true"})
	}
	if family := strings.TrimSpace(input.ArtifactFamily); family != "" {
		env = append(env, corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_ARTIFACT_FAMILY", Value: family})
	}
	return env
}

func buildRuntimeEnv(input Input, options Options, modelPath string) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: ModelPathEnv, Value: modelPath},
		{Name: ModelDigestEnv, Value: strings.TrimSpace(input.Artifact.Digest)},
	}
	if family := strings.TrimSpace(input.ArtifactFamily); family != "" {
		env = append(env, corev1.EnvVar{Name: ModelFamilyEnv, Value: family})
	}
	return env
}
