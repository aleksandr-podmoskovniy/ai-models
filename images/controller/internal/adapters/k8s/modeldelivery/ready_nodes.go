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

	k8snodecacheruntime "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deliveryGate struct {
	Ready   bool
	Reason  DeliveryGateReason
	Message string
}

func (s *Service) deliveryGateForTemplate(
	ctx context.Context,
	topologyKind CacheTopologyKind,
	input Input,
	template *corev1.PodTemplateSpec,
) (deliveryGate, error) {
	if topologyKind != CacheTopologyDirect {
		return deliveryGate{Ready: true}, nil
	}
	managed := NormalizeManagedCacheOptions(s.options.ManagedCache)
	artifacts := desiredArtifactsFromInput(input)
	if err := managedCacheCapacityError(managed, artifacts); err != nil {
		return deliveryGate{
			Reason:  DeliveryGateReasonInsufficientNodeCacheCapacity,
			Message: fmt.Sprintf("SharedDirect node cache cannot fit requested model artifacts: %v", err),
		}, nil
	}
	gate, err := s.managedCacheGateForTemplate(ctx, managed, artifacts, template)
	if err != nil {
		return deliveryGate{}, err
	}
	return gate, nil
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

func (s *Service) managedCacheGateForTemplate(
	ctx context.Context,
	managed ManagedCacheOptions,
	artifacts []nodecache.DesiredArtifact,
	template *corev1.PodTemplateSpec,
) (deliveryGate, error) {
	if !managed.Enabled || len(managed.NodeSelector) == 0 {
		return deliveryGate{Ready: true}, nil
	}
	nodes := &corev1.NodeList{}
	if err := s.client.List(ctx, nodes, client.MatchingLabels(managed.NodeSelector)); err != nil {
		return deliveryGate{}, err
	}
	summaries, err := s.managedCacheRuntimeSummariesForGate(ctx, managed)
	if err != nil {
		return deliveryGate{}, err
	}

	return managedCacheGateForNodes(nodes.Items, template, managed.CapacityBytes, artifacts, summaries)
}

func (s *Service) managedCacheRuntimeSummariesForGate(
	ctx context.Context,
	managed ManagedCacheOptions,
) (map[string]nodecache.RuntimeUsageSummary, error) {
	if managed.CapacityBytes <= 0 {
		return nil, nil
	}
	return s.managedCacheRuntimeSummaries(ctx, managed)
}

func managedCacheGateForNodes(
	nodes []corev1.Node,
	template *corev1.PodTemplateSpec,
	capacityBytes int64,
	artifacts []nodecache.DesiredArtifact,
	summaries map[string]nodecache.RuntimeUsageSummary,
) (deliveryGate, error) {
	matchedNodes := 0
	summarizedNodes := 0
	var bestMissingBytes, bestAvailableBytes int64
	for index := range nodes {
		node := nodes[index]
		if !nodeFitsTemplate(node, template) {
			continue
		}
		matchedNodes++
		if capacityBytes <= 0 {
			return deliveryGate{Ready: true}, nil
		}

		summary, found := summaries[node.Name]
		if !found {
			continue
		}
		summarizedNodes++
		missingBytes, err := nodecache.MissingSizeBytes(summary, artifacts)
		if err != nil {
			return deliveryGate{}, err
		}
		if missingBytes <= summary.AvailableBytes {
			return deliveryGate{Ready: true}, nil
		}
		if bestMissingBytes == 0 || missingBytes-summary.AvailableBytes < bestMissingBytes-bestAvailableBytes {
			bestMissingBytes = missingBytes
			bestAvailableBytes = summary.AvailableBytes
		}
	}
	if matchedNodes == 0 {
		return deliveryGate{
			Reason:  DeliveryGateReasonNoReadyNodeCacheRuntime,
			Message: "SharedDirect node cache has no ready node matching workload scheduling constraints",
		}, nil
	}
	if summarizedNodes < matchedNodes {
		return deliveryGate{
			Reason:  DeliveryGateReasonNoReadyNodeCacheRuntime,
			Message: "SharedDirect node cache is waiting for per-node usage summary on a matching ready node",
		}, nil
	}
	return deliveryGate{
		Reason: DeliveryGateReasonInsufficientNodeCacheCapacity,
		Message: fmt.Sprintf(
			"SharedDirect node cache has no matching ready node with enough free cache space: missingBytes=%d availableBytes=%d",
			bestMissingBytes,
			bestAvailableBytes,
		),
	}, nil
}

func (s *Service) managedCacheRuntimeSummaries(ctx context.Context, managed ManagedCacheOptions) (map[string]nodecache.RuntimeUsageSummary, error) {
	namespace := strings.TrimSpace(managed.RuntimeNamespace)
	if namespace == "" {
		namespace = strings.TrimSpace(s.options.RegistrySourceNamespace)
	}
	if namespace == "" {
		return nil, nil
	}
	pods := &corev1.PodList{}
	if err := s.client.List(ctx, pods,
		client.InNamespace(namespace),
		client.MatchingLabels{k8snodecacheruntime.ManagedLabelKey: k8snodecacheruntime.ManagedLabelValue},
	); err != nil {
		return nil, err
	}
	summaries := make(map[string]nodecache.RuntimeUsageSummary, len(pods.Items))
	for index := range pods.Items {
		pod := &pods.Items[index]
		if !runtimePodReady(pod) {
			continue
		}
		summary, found, err := k8snodecacheruntime.UsageSummaryFromPod(pod)
		if err != nil {
			return nil, err
		}
		if found {
			summaries[summary.NodeName] = summary
		}
	}
	return summaries, nil
}

func runtimePodReady(pod *corev1.Pod) bool {
	if pod == nil || pod.DeletionTimestamp != nil {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
