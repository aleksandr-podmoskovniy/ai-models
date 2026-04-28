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
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type CacheMount struct {
	VolumeName string
	MountPath  string
}

func detectCacheMount(template *corev1.PodTemplateSpec, mountPath string) (CacheMount, error) {
	resolved, found, err := resolveCacheMount(template, mountPath)
	if err != nil {
		return CacheMount{}, err
	}
	if !found {
		return CacheMount{}, NewWorkloadContractError("runtime delivery annotated workload must mount writable model cache at %q", normalizeMountPath(mountPath))
	}
	return resolved, nil
}

func resolveCacheMount(template *corev1.PodTemplateSpec, mountPath string) (CacheMount, bool, error) {
	if template == nil {
		return CacheMount{}, false, errors.New("runtime delivery pod template must not be nil")
	}
	if len(template.Spec.Containers) == 0 {
		return CacheMount{}, false, errors.New("runtime delivery pod template must contain at least one container")
	}

	expectedPath := normalizeMountPath(mountPath)
	resolved := CacheMount{MountPath: expectedPath}
	for _, container := range template.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if normalizeMountPath(mount.MountPath) != expectedPath {
				continue
			}
			if strings.TrimSpace(resolved.VolumeName) == "" {
				resolved.VolumeName = mount.Name
				continue
			}
			if mount.Name != resolved.VolumeName {
				return CacheMount{}, false, NewWorkloadContractError("runtime delivery cache mount %q must reference a single backing volume", expectedPath)
			}
		}
	}
	if strings.TrimSpace(resolved.VolumeName) == "" {
		return CacheMount{}, false, nil
	}
	return resolved, true, nil
}

func normalizeMountPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	return path.Clean(value)
}
