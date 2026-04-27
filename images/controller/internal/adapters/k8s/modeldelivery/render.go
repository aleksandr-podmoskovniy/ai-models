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
	Bindings                   []BindingInput
	RegistryAccess             ociregistry.Projection
	RuntimeImagePullSecretName string
	CacheMount                 CacheMount
	TopologyKind               CacheTopologyKind
	Coordination               Coordination
}

type BindingInput struct {
	Alias          string
	Artifact       publication.PublishedArtifact
	ArtifactFamily string
}

type Rendered struct {
	InitContainers            []corev1.Container
	InitContainerName         string
	InitContainerNames        []string
	RuntimeEnv                []corev1.EnvVar
	Volumes                   []corev1.Volume
	RuntimeVolumeMounts       []corev1.VolumeMount
	ImagePullSecrets          []corev1.LocalObjectReference
	ImagePullSecretNamesPrune []string
	ModelPath                 string
	ArtifactURI               string
	ArtifactFamily            string
	ResolvedModels            string
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
	bindings, aliasContract, err := renderBindings(input)
	if err != nil {
		return Rendered{}, err
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

	if aliasContract {
		return renderAliasBindings(input, options, bindings)
	}

	modelPath := ModelPath(options)
	if input.TopologyKind == CacheTopologySharedPVC {
		modelPath = nodecache.SharedArtifactModelPath(options.CacheMountPath, input.Artifact.Digest)
	}
	if input.TopologyKind == CacheTopologyDirect {
		return Rendered{
			InitContainerName:         options.InitContainerName,
			RuntimeEnv:                buildRuntimeEnv(input, options, modelPath),
			ImagePullSecretNamesPrune: buildImagePullSecretNamesPrune(input.RuntimeImagePullSecretName),
			ModelPath:                 modelPath,
			ArtifactURI:               strings.TrimSpace(input.Artifact.URI),
			ArtifactFamily:            strings.TrimSpace(input.ArtifactFamily),
		}, nil
	}

	initContainers := []corev1.Container{
		buildMaterializerContainer(options.InitContainerName, input, options, bindings[0], input.TopologyKind == CacheTopologySharedPVC, ""),
	}
	return Rendered{
		InitContainerName:  options.InitContainerName,
		InitContainerNames: initContainerNames(initContainers),
		InitContainers:     initContainers,
		RuntimeEnv:         buildRuntimeEnv(input, options, modelPath),
		Volumes:            ociregistry.Volumes(input.RegistryAccess.CASecretName),
		ImagePullSecrets:   buildImagePullSecrets(input.RuntimeImagePullSecretName),
		ModelPath:          modelPath,
		ArtifactURI:        strings.TrimSpace(input.Artifact.URI),
		ArtifactFamily:     strings.TrimSpace(input.ArtifactFamily),
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

func buildMaterializerContainer(name string, input Input, options Options, binding BindingInput, sharedStore bool, alias string) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           options.RuntimeImage,
		ImagePullPolicy: options.ImagePullPolicy,
		Args:            []string{"materialize-artifact"},
		Env:             buildMaterializeEnv(input, options, binding, sharedStore, alias),
		VolumeMounts:    buildMaterializerVolumeMounts(input, options),
	}
}

func buildMaterializerVolumeMounts(input Input, options Options) []corev1.VolumeMount {
	return append([]corev1.VolumeMount{{
		Name:      input.CacheMount.VolumeName,
		MountPath: options.CacheMountPath,
	}}, ociregistry.VolumeMounts(input.RegistryAccess.CASecretName)...)
}

func buildMaterializeEnv(input Input, options Options, binding BindingInput, sharedStore bool, alias string) []corev1.EnvVar {
	env := ociregistry.Env(options.OCIInsecure, input.RegistryAccess.AuthSecretName, input.RegistryAccess.CASecretName)
	env = append(env,
		corev1.EnvVar{Name: LogFormatEnv, Value: options.LogFormat},
		corev1.EnvVar{Name: LogLevelEnv, Value: options.LogLevel},
		corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_ARTIFACT_URI", Value: binding.Artifact.URI},
		corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_ARTIFACT_DIGEST", Value: binding.Artifact.Digest},
		corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_CACHE_ROOT", Value: options.CacheMountPath},
	)
	if sharedStore {
		env = append(env, corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_SHARED_STORE", Value: "true"})
	}
	if alias = strings.TrimSpace(alias); alias != "" {
		env = append(env, corev1.EnvVar{Name: "AI_MODELS_MATERIALIZE_MODEL_ALIAS", Value: alias})
	}
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
	if family := strings.TrimSpace(binding.ArtifactFamily); family != "" {
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
