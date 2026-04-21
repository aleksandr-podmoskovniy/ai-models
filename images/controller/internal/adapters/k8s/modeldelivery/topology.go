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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type CacheTopologyKind string

const (
	CacheTopologyPerPod    CacheTopologyKind = "PerPod"
	CacheTopologySharedPVC CacheTopologyKind = "SharedPVC"
)

type TopologyHints struct {
	ReplicaCount         int32
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

type CacheTopology struct {
	Kind           CacheTopologyKind
	CacheMount     CacheMount
	ClaimName      string
	DeliveryMode   DeliveryMode
	DeliveryReason DeliveryReason
}

func detectCacheTopology(template *corev1.PodTemplateSpec, hints TopologyHints, mountPath, managedVolumeName string) (CacheTopology, error) {
	cacheMount, err := detectCacheMount(template, mountPath)
	if err != nil {
		return CacheTopology{}, err
	}

	hints = normalizeTopologyHints(hints)
	if claimTemplateExists(hints.VolumeClaimTemplates, cacheMount.VolumeName) {
		return CacheTopology{
			Kind:           CacheTopologyPerPod,
			CacheMount:     cacheMount,
			ClaimName:      cacheMount.VolumeName,
			DeliveryMode:   DeliveryModeMaterializeBridge,
			DeliveryReason: DeliveryReasonStatefulSetClaimTemplate,
		}, nil
	}

	volume, found := findVolumeByName(template.Spec.Volumes, cacheMount.VolumeName)
	if !found {
		return CacheTopology{}, fmt.Errorf("runtime delivery cache volume %q must be declared in pod template or provided via claim template", cacheMount.VolumeName)
	}

	switch {
	case volume.PersistentVolumeClaim != nil:
		claimName := strings.TrimSpace(volume.PersistentVolumeClaim.ClaimName)
		if claimName == "" {
			return CacheTopology{}, fmt.Errorf("runtime delivery cache volume %q must reference a non-empty persistentVolumeClaim name", cacheMount.VolumeName)
		}
		return CacheTopology{
			Kind:           CacheTopologySharedPVC,
			CacheMount:     cacheMount,
			ClaimName:      claimName,
			DeliveryMode:   DeliveryModeSharedPVCBridge,
			DeliveryReason: DeliveryReasonWorkloadSharedPersistentVolume,
		}, nil
	case volume.EmptyDir != nil || volume.Ephemeral != nil:
		reason := DeliveryReasonWorkloadCacheVolume
		if strings.TrimSpace(managedVolumeName) != "" && volume.Name == strings.TrimSpace(managedVolumeName) {
			reason = DeliveryReasonManagedBridgeVolume
		}
		return CacheTopology{
			Kind:           CacheTopologyPerPod,
			CacheMount:     cacheMount,
			DeliveryMode:   DeliveryModeMaterializeBridge,
			DeliveryReason: reason,
		}, nil
	default:
		return CacheTopology{}, fmt.Errorf("runtime delivery cache volume %q uses unsupported source; expected persistentVolumeClaim, emptyDir, ephemeral, or StatefulSet claim template", cacheMount.VolumeName)
	}
}

func normalizeTopologyHints(hints TopologyHints) TopologyHints {
	if hints.ReplicaCount <= 0 {
		hints.ReplicaCount = 1
	}
	return hints
}

func claimTemplateExists(templates []corev1.PersistentVolumeClaim, volumeName string) bool {
	volumeName = strings.TrimSpace(volumeName)
	if volumeName == "" {
		return false
	}
	for _, template := range templates {
		if strings.TrimSpace(template.Name) == volumeName {
			return true
		}
	}
	return false
}

func findVolumeByName(volumes []corev1.Volume, name string) (corev1.Volume, bool) {
	if strings.TrimSpace(name) == "" {
		return corev1.Volume{}, false
	}
	for _, volume := range volumes {
		if volume.Name == name {
			return volume, true
		}
	}
	return corev1.Volume{}, false
}

func validateTopologyHints(hints TopologyHints) error {
	if hints.ReplicaCount < 0 {
		return errors.New("runtime delivery replica count must not be negative")
	}
	return nil
}
