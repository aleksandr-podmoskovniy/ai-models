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
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CoordinationModeNone   = ""
	CoordinationModeShared = "shared-cache"
)

type Coordination struct {
	Mode string
}

func (s *Service) resolveCoordination(
	ctx context.Context,
	targetNamespace string,
	topology CacheTopology,
	hints TopologyHints,
	digest string,
) (Coordination, error) {
	hints = normalizeTopologyHints(hints)
	if topology.Kind != CacheTopologySharedDirect || hints.ReplicaCount <= 1 {
		return Coordination{}, nil
	}

	claimName := strings.TrimSpace(topology.ClaimName)
	if claimName == "" {
		return Coordination{}, fmt.Errorf("runtime delivery shared cache topology requires persistentVolumeClaim name")
	}

	claim := &corev1.PersistentVolumeClaim{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: targetNamespace, Name: claimName}, claim); err != nil {
		return Coordination{}, err
	}
	if !supportsReadWriteMany(claim.Spec.AccessModes) {
		return Coordination{}, fmt.Errorf("runtime delivery shared persistentVolumeClaim %q for replicas > 1 must support ReadWriteMany", claimName)
	}

	return Coordination{
		Mode: CoordinationModeShared,
	}, nil
}

func supportsReadWriteMany(modes []corev1.PersistentVolumeAccessMode) bool {
	for _, mode := range modes {
		if mode == corev1.ReadWriteMany {
			return true
		}
	}
	return false
}
