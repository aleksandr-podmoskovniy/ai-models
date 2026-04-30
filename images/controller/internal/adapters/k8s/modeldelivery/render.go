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

	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
)

type Input struct {
	Artifact                  publication.PublishedArtifact
	ArtifactFamily            string
	Bindings                  []BindingInput
	LegacyImagePullSecretName string
	CacheMount                CacheMount
	SharedPVCClaimName        string
	TopologyKind              CacheTopologyKind
}

type BindingInput struct {
	Name           string
	Artifact       publication.PublishedArtifact
	ArtifactFamily string
}

type Rendered struct {
	RuntimeEnv                []corev1.EnvVar
	Volumes                   []corev1.Volume
	RuntimeVolumeMounts       []corev1.VolumeMount
	LegacyInitContainerName   string
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
	bindings, _, err := renderBindings(input)
	if err != nil {
		return Rendered{}, err
	}
	switch input.TopologyKind {
	case CacheTopologyDirect:
	case CacheTopologySharedPVC:
		if strings.TrimSpace(input.SharedPVCClaimName) == "" {
			return Rendered{}, errors.New("runtime delivery SharedPVC claim name must not be empty")
		}
	default:
		return Rendered{}, NewWorkloadContractError("runtime delivery supports only SharedDirect or SharedPVC delivery")
	}
	if strings.TrimSpace(input.CacheMount.VolumeName) == "" {
		return Rendered{}, errors.New("runtime delivery cache volume name must not be empty")
	}
	if normalizeMountPath(input.CacheMount.MountPath) != normalizeMountPath(options.CacheMountPath) {
		return Rendered{}, errors.New("runtime delivery cache mount contract is inconsistent")
	}

	return renderModelBindings(input, options, bindings)
}

func buildImagePullSecretNamesPrune(secretName string) []string {
	if strings.TrimSpace(secretName) == "" {
		return nil
	}
	return []string{strings.TrimSpace(secretName)}
}
