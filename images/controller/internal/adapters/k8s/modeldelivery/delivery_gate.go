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
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type deliveryGate struct {
	Ready   bool
	Reason  DeliveryGateReason
	Message string
}

func (s *Service) deliveryGateForTemplate(
	topology CacheTopology,
	input Input,
	pvcState sharedPVCState,
) (deliveryGate, error) {
	switch topology.Kind {
	case CacheTopologyDirect:
		return s.sharedDirectGate(input)
	case CacheTopologySharedPVC:
		return sharedPVCGate(pvcState), nil
	default:
		return deliveryGate{Ready: true}, nil
	}
}

func (s *Service) sharedDirectGate(input Input) (deliveryGate, error) {
	managed := NormalizeManagedCacheOptions(s.options.ManagedCache)
	if !managed.Enabled {
		return deliveryGate{
			Reason:  DeliveryGateReasonNodeCacheDeliveryDisabled,
			Message: "SharedDirect node-cache delivery is disabled",
		}, nil
	}
	artifacts := desiredArtifactsFromInput(input)
	if err := managedCacheCapacityError(managed, artifacts); err != nil {
		return deliveryGate{
			Reason:  DeliveryGateReasonInsufficientNodeCacheCapacity,
			Message: fmt.Sprintf("SharedDirect node cache cannot fit requested model artifacts: %v", err),
		}, nil
	}
	return deliveryGate{Ready: true}, nil
}

func sharedPVCGate(state sharedPVCState) deliveryGate {
	if strings.TrimSpace(state.ClaimName) == "" {
		return deliveryGate{
			Reason:  DeliveryGateReasonSharedPVCStorageClassMissing,
			Message: "SharedPVC delivery requires sharedPVC.storageClassName with an RWX StorageClass",
		}
	}
	if !state.Bound {
		return deliveryGate{
			Reason:  DeliveryGateReasonSharedPVCClaimPending,
			Message: "SharedPVC delivery is waiting for the controller-owned RWX PVC to become Bound",
		}
	}
	return deliveryGate{
		Reason:  DeliveryGateReasonSharedPVCMaterializerPending,
		Message: "SharedPVC delivery is waiting for digest-scoped materialization to complete",
	}
}

func managedCacheCapacityError(managed ManagedCacheOptions, artifacts []nodecache.DesiredArtifact) error {
	if !managed.Enabled || managed.CapacityBytes <= 0 {
		return nil
	}
	return nodecache.ValidateDesiredArtifactsFit(managed.CapacityBytes, artifacts)
}

func desiredArtifactsFromInput(input Input) []nodecache.DesiredArtifact {
	artifacts := []nodecache.DesiredArtifact{desiredArtifactFromBinding(input.Artifact, input.ArtifactFamily)}
	if len(input.Bindings) > 0 {
		artifacts = make([]nodecache.DesiredArtifact, 0, len(input.Bindings))
		for _, binding := range input.Bindings {
			artifacts = append(artifacts, desiredArtifactFromBinding(binding.Artifact, binding.ArtifactFamily))
		}
	}
	return artifacts
}

func desiredArtifactFromBinding(artifact publication.PublishedArtifact, family string) nodecache.DesiredArtifact {
	return nodecache.DesiredArtifact{
		ArtifactURI: artifact.URI,
		Digest:      artifact.Digest,
		Family:      family,
		SizeBytes:   artifact.SizeBytes,
	}
}
