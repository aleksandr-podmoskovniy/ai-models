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
	"encoding/json"
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
)

func renderBindings(input Input) ([]BindingInput, bool, error) {
	if len(input.Bindings) == 0 {
		return []BindingInput{{
			Alias:          "model",
			Artifact:       input.Artifact,
			ArtifactFamily: input.ArtifactFamily,
		}}, false, nil
	}
	bindings := make([]BindingInput, 0, len(input.Bindings))
	for _, binding := range input.Bindings {
		if err := nodecache.ValidateModelAlias(binding.Alias); err != nil {
			return nil, false, err
		}
		if err := binding.Artifact.Validate(); err != nil {
			return nil, false, err
		}
		if strings.TrimSpace(binding.Artifact.Digest) == "" {
			return nil, false, errors.New("runtime delivery artifact digest must not be empty")
		}
		bindings = append(bindings, binding)
	}
	return bindings, true, nil
}

func renderAliasBindings(input Input, options Options, bindings []BindingInput) (Rendered, error) {
	runtimeEntries := runtimeModelEntries(options, bindings)
	resolvedEntries := resolvedModelEntries(bindings)
	modelsJSON, err := json.Marshal(resolvedEntries)
	if err != nil {
		return Rendered{}, err
	}

	rendered := Rendered{
		RuntimeEnv:              buildAliasRuntimeEnv(options, runtimeEntries),
		LegacyInitContainerName: options.LegacyInitContainerName,
		ModelPath:               runtimeEntries[0].Path,
		ArtifactURI:             strings.TrimSpace(bindings[0].Artifact.URI),
		ArtifactFamily:          strings.TrimSpace(bindings[0].ArtifactFamily),
		ResolvedModels:          string(modelsJSON),
	}
	rendered.RuntimeVolumeMounts = buildAliasVolumeMounts(input.CacheMount.VolumeName, options, bindings)
	rendered.ImagePullSecretNamesPrune = buildImagePullSecretNamesPrune(input.LegacyImagePullSecretName)
	return rendered, nil
}

type runtimeModelEntry struct {
	Alias  string `json:"alias"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Family string `json:"family,omitempty"`
}

type resolvedModelEntry struct {
	Alias     string `json:"alias"`
	URI       string `json:"uri"`
	Digest    string `json:"digest"`
	Family    string `json:"family,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}

func runtimeModelEntries(options Options, bindings []BindingInput) []runtimeModelEntry {
	entries := make([]runtimeModelEntry, 0, len(bindings))
	for _, binding := range bindings {
		entries = append(entries, runtimeModelEntry{
			Alias:  binding.Alias,
			Path:   NamedModelPath(options, binding.Alias),
			Digest: strings.TrimSpace(binding.Artifact.Digest),
			Family: strings.TrimSpace(binding.ArtifactFamily),
		})
	}
	return entries
}

func resolvedModelEntries(bindings []BindingInput) []resolvedModelEntry {
	entries := make([]resolvedModelEntry, 0, len(bindings))
	for _, binding := range bindings {
		entries = append(entries, resolvedModelEntry{
			Alias:     binding.Alias,
			URI:       strings.TrimSpace(binding.Artifact.URI),
			Digest:    strings.TrimSpace(binding.Artifact.Digest),
			Family:    strings.TrimSpace(binding.ArtifactFamily),
			SizeBytes: binding.Artifact.SizeBytes,
		})
	}
	return entries
}

func buildAliasRuntimeEnv(options Options, entries []runtimeModelEntry) []corev1.EnvVar {
	modelsJSON, _ := json.Marshal(entries)
	env := []corev1.EnvVar{
		{Name: ModelPathEnv, Value: entries[0].Path},
		{Name: ModelDigestEnv, Value: entries[0].Digest},
		{Name: ModelsDirEnv, Value: ModelsDirPath(options)},
		{Name: ModelsEnv, Value: string(modelsJSON)},
	}
	if entries[0].Family != "" {
		env = append(env, corev1.EnvVar{Name: ModelFamilyEnv, Value: entries[0].Family})
	}
	for _, entry := range entries {
		env = append(env,
			corev1.EnvVar{Name: NamedModelPathEnv(entry.Alias), Value: entry.Path},
			corev1.EnvVar{Name: NamedModelDigestEnv(entry.Alias), Value: entry.Digest},
		)
		if entry.Family != "" {
			env = append(env, corev1.EnvVar{Name: NamedModelFamilyEnv(entry.Alias), Value: entry.Family})
		}
	}
	return env
}

func buildAliasVolumeMounts(volumeNamePrefix string, options Options, bindings []BindingInput) []corev1.VolumeMount {
	if strings.TrimSpace(volumeNamePrefix) == "" {
		volumeNamePrefix = DefaultManagedCacheName
	}
	mounts := make([]corev1.VolumeMount, 0, len(bindings))
	for _, binding := range bindings {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      managedModelVolumeName(volumeNamePrefix, binding.Alias),
			MountPath: NamedModelPath(options, binding.Alias),
			ReadOnly:  true,
		})
	}
	return mounts
}
