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
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func (s *Service) pruneManagedCacheTemplateState(
	template *corev1.PodTemplateSpec,
	topology CacheTopology,
	rendered Rendered,
	aliasContract bool,
) {
	managed := NormalizeManagedCacheOptions(s.options.ManagedCache)
	if !managed.Enabled {
		return
	}
	PruneManagedCacheTemplateState(template, managed.VolumeName, managedCacheKeepNames(topology, rendered, aliasContract))
}

func managedCacheKeepNames(topology CacheTopology, rendered Rendered, aliasContract bool) []string {
	if topology.Kind != CacheTopologyDirect {
		return nil
	}
	if aliasContract {
		return volumeNames(rendered.Volumes)
	}
	if strings.TrimSpace(topology.CacheMount.VolumeName) == "" {
		return nil
	}
	return []string{strings.TrimSpace(topology.CacheMount.VolumeName)}
}
