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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	sharedPVCManagedLabel  = "ai.deckhouse.io/shared-pvc"
	sharedPVCOwnerUIDLabel = "ai.deckhouse.io/workload-uid"
	sharedPVCSetHashLabel  = "ai.deckhouse.io/model-set-hash"
	sharedPVCNamePrefix    = "ai-models-cache"
	sharedPVCMinSizeBytes  = int64(1 << 30)
)

type sharedPVCState struct {
	ClaimName string
	Bound     bool
}

func sharedPVCClaimName(owner client.Object, bindings []ModelBinding) string {
	sum := sha1.Sum([]byte(sharedPVCIdentity(owner, bindings)))
	return sharedPVCNamePrefix + "-" + hex.EncodeToString(sum[:])[:12]
}

func sharedPVCIdentity(owner client.Object, bindings []ModelBinding) string {
	parts := []string{
		strings.TrimSpace(owner.GetNamespace()),
		strings.TrimSpace(owner.GetName()),
		string(owner.GetUID()),
	}
	models := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		models = append(models, strings.TrimSpace(binding.Name)+"@"+strings.TrimSpace(binding.Artifact.Digest))
	}
	sort.Strings(models)
	parts = append(parts, models...)
	return strings.Join(parts, "\n")
}

func (s *Service) ensureSharedPVC(
	ctx context.Context,
	owner client.Object,
	bindings []ModelBinding,
	claimName string,
) (sharedPVCState, error) {
	shared := NormalizeSharedPVCOptions(s.options.SharedPVC)
	if strings.TrimSpace(shared.StorageClassName) == "" {
		return sharedPVCState{}, nil
	}
	size, err := sharedPVCRequestedStorage(bindings)
	if err != nil {
		return sharedPVCState{}, err
	}

	desired := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: owner.GetNamespace(),
			Labels: map[string]string{
				sharedPVCManagedLabel:  "true",
				sharedPVCOwnerUIDLabel: string(owner.GetUID()),
				sharedPVCSetHashLabel:  strings.TrimPrefix(claimName, sharedPVCNamePrefix+"-"),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			StorageClassName: &shared.StorageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: *resource.NewQuantity(size, resource.BinarySI)},
			},
		},
	}
	if err := controllerutil.SetControllerReference(owner, &desired, s.scheme); err != nil {
		return sharedPVCState{}, err
	}

	current := &corev1.PersistentVolumeClaim{}
	key := client.ObjectKey{Namespace: desired.Namespace, Name: desired.Name}
	if err := s.client.Get(ctx, key, current); err != nil {
		if !apierrors.IsNotFound(err) {
			return sharedPVCState{}, err
		}
		if err := s.client.Create(ctx, &desired); err != nil {
			return sharedPVCState{}, err
		}
		if err := s.deleteStaleSharedPVCs(ctx, owner, desired.Name); err != nil {
			return sharedPVCState{}, err
		}
		return sharedPVCState{ClaimName: desired.Name}, nil
	}
	if err := validateSharedPVC(current, shared.StorageClassName); err != nil {
		return sharedPVCState{}, err
	}
	if err := s.deleteStaleSharedPVCs(ctx, owner, desired.Name); err != nil {
		return sharedPVCState{}, err
	}
	return sharedPVCState{
		ClaimName: desired.Name,
		Bound:     current.Status.Phase == corev1.ClaimBound,
	}, nil
}

func sharedPVCRequestedStorage(bindings []ModelBinding) (int64, error) {
	var total int64
	for _, binding := range bindings {
		if binding.Artifact.SizeBytes <= 0 {
			return 0, fmt.Errorf("SharedPVC delivery requires published sizeBytes for model %q", binding.Name)
		}
		total += binding.Artifact.SizeBytes
	}
	// Keep a small filesystem/headroom budget without exposing a user knob.
	withHeadroom := total + total/10
	if withHeadroom < sharedPVCMinSizeBytes {
		return sharedPVCMinSizeBytes, nil
	}
	return withHeadroom, nil
}

func validateSharedPVC(claim *corev1.PersistentVolumeClaim, storageClassName string) error {
	if claim == nil {
		return nil
	}
	if claim.Spec.StorageClassName == nil || strings.TrimSpace(*claim.Spec.StorageClassName) != strings.TrimSpace(storageClassName) {
		return NewWorkloadContractError("SharedPVC claim %q uses unexpected storageClassName", claim.Name)
	}
	for _, mode := range claim.Spec.AccessModes {
		if mode == corev1.ReadWriteMany {
			return nil
		}
	}
	return NewWorkloadContractError("SharedPVC claim %q must use ReadWriteMany access mode", claim.Name)
}

func (s *Service) deleteStaleSharedPVCs(ctx context.Context, owner client.Object, keepName string) error {
	if strings.TrimSpace(string(owner.GetUID())) == "" {
		return nil
	}
	claims := &corev1.PersistentVolumeClaimList{}
	if err := s.client.List(
		ctx,
		claims,
		client.InNamespace(owner.GetNamespace()),
		client.MatchingLabels{
			sharedPVCManagedLabel:  "true",
			sharedPVCOwnerUIDLabel: string(owner.GetUID()),
		},
	); err != nil {
		return err
	}
	for index := range claims.Items {
		claim := &claims.Items[index]
		if claim.Name == keepName {
			continue
		}
		if err := s.client.Delete(ctx, claim); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (s *Service) DeleteSharedPVCsForOwner(ctx context.Context, owner client.Object) error {
	if s == nil || owner == nil || strings.TrimSpace(string(owner.GetUID())) == "" {
		return nil
	}
	return s.deleteStaleSharedPVCs(ctx, owner, "")
}

func RemoveSharedPVCTemplateState(template *corev1.PodTemplateSpec, options ServiceOptions) bool {
	if template == nil {
		return false
	}
	shared := NormalizeSharedPVCOptions(options.SharedPVC)
	changed := false
	var removed bool
	template.Spec.Containers, removed = removeVolumeMountsByName(template.Spec.Containers, shared.VolumeName)
	if removed {
		changed = true
	}
	template.Spec.InitContainers, removed = removeVolumeMountsByName(template.Spec.InitContainers, shared.VolumeName)
	if removed {
		changed = true
	}
	template.Spec.Volumes, removed = removeModelDeliveryVolumeByName(template.Spec.Volumes, shared.VolumeName)
	return changed || removed
}

func HasSharedPVCTemplateState(template *corev1.PodTemplateSpec, options ServiceOptions) bool {
	if template == nil {
		return false
	}
	shared := NormalizeSharedPVCOptions(options.SharedPVC)
	if hasVolumeByName(template.Spec.Volumes, shared.VolumeName) {
		return true
	}
	return containersMountManagedVolume(template.Spec.Containers, shared.VolumeName) ||
		containersMountManagedVolume(template.Spec.InitContainers, shared.VolumeName)
}

func removeVolumeMountsByName(containers []corev1.Container, name string) ([]corev1.Container, bool) {
	removed := false
	for index := range containers {
		filtered := containers[index].VolumeMounts[:0]
		for _, mount := range containers[index].VolumeMounts {
			if mount.Name == name {
				removed = true
				continue
			}
			filtered = append(filtered, mount)
		}
		containers[index].VolumeMounts = filtered
	}
	return containers, removed
}

func hasVolumeByName(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func removeModelDeliveryVolumeByName(volumes []corev1.Volume, name string) ([]corev1.Volume, bool) {
	removed := false
	filtered := volumes[:0]
	for _, volume := range volumes {
		if volume.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, volume)
	}
	return filtered, removed
}
