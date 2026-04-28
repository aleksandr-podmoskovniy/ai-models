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
	"encoding/json"
	"testing"
	"time"

	k8snodecacheruntime "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceKeepsSchedulingGateWhenReadyNodeDoesNotMatchWorkloadSelector(t *testing.T) {
	t.Parallel()

	service, owner := newManagedSharedDirectService(t, readyNode())
	template := podTemplateWithoutCacheMount("runtime")
	template.Spec.NodeSelector = map[string]string{"node.deckhouse.io/pool": "gpu"}

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if got, want := result.GateReason, DeliveryGateReasonNoReadyNodeCacheRuntime; got != want {
		t.Fatalf("gate reason = %q, want %q", got, want)
	}
	if !HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate while ready node does not match workload selector")
	}
}

func TestServiceKeepsSchedulingGateWhenManagedCacheCannotFitArtifact(t *testing.T) {
	t.Parallel()

	service, owner := newManagedSharedDirectService(t, readyNode())
	service.options.ManagedCache.CapacityBytes = 10
	template := podTemplateWithoutCacheMount("runtime")

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if got, want := result.GateReason, DeliveryGateReasonInsufficientNodeCacheCapacity; got != want {
		t.Fatalf("gate reason = %q, want %q", got, want)
	}
	if !HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate while artifact does not fit managed cache")
	}
}

func TestServiceKeepsSchedulingGateWhenReadyNodeHasInsufficientFreeCache(t *testing.T) {
	t.Parallel()

	service, owner := newManagedSharedDirectService(t, readyNode(), readyRuntimePodWithUsage(t, "worker-a", nodecache.RuntimeUsageSummary{
		Version:        nodecache.RuntimeUsageSummaryVersion,
		NodeName:       "worker-a",
		LimitBytes:     100,
		UsedBytes:      95,
		AvailableBytes: 5,
		UpdatedAt:      time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
	}))
	service.options.ManagedCache.CapacityBytes = 100
	template := podTemplateWithoutCacheMount("runtime")

	result, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if got, want := result.GateReason, DeliveryGateReasonInsufficientNodeCacheCapacity; got != want {
		t.Fatalf("gate reason = %q, want %q", got, want)
	}
	if !HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate while matching node has insufficient free cache")
	}
}

func TestServiceRemovesSchedulingGateWhenReadyNodeAlreadyHasRequestedDigest(t *testing.T) {
	t.Parallel()

	artifact := publishedArtifact()
	service, owner := newManagedSharedDirectService(t, readyNode(), readyRuntimePodWithUsage(t, "worker-a", nodecache.RuntimeUsageSummary{
		Version:        nodecache.RuntimeUsageSummaryVersion,
		NodeName:       "worker-a",
		LimitBytes:     100,
		UsedBytes:      95,
		AvailableBytes: 5,
		ReadyArtifacts: []nodecache.RuntimeUsageArtifact{{
			Digest:    artifact.Digest,
			SizeBytes: artifact.SizeBytes,
		}},
		UpdatedAt: time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
	}))
	service.options.ManagedCache.CapacityBytes = 100
	template := podTemplateWithoutCacheMount("runtime")

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: artifact,
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if HasSchedulingGate(template) {
		t.Fatalf("did not expect scheduling gate when requested digest is already cached")
	}
}

func TestServiceKeepsSchedulingGateWhenReadyNodeDoesNotMatchRequiredAffinity(t *testing.T) {
	t.Parallel()

	service, owner := newManagedSharedDirectService(t, readyNode())
	template := podTemplateWithoutCacheMount("runtime")
	template.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{
				MatchExpressions: []corev1.NodeSelectorRequirement{{
					Key:      "accelerator.deckhouse.io/class",
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"gpu"},
				}},
			}},
		},
	}}

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if !HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate while ready node does not match required node affinity")
	}
}

func TestServiceKeepsSchedulingGateWhenReadyNodeHasUntoleratedHardTaint(t *testing.T) {
	t.Parallel()

	service, owner := newManagedSharedDirectService(t, readyNodeWithTaint(corev1.Taint{
		Key:    "node-cache.deckhouse.io/maintenance",
		Value:  "true",
		Effect: corev1.TaintEffectNoSchedule,
	}))
	template := podTemplateWithoutCacheMount("runtime")

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if !HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate while ready node has untolerated hard taint")
	}
}

func TestServiceRemovesSchedulingGateWhenReadyNodeTaintIsTolerated(t *testing.T) {
	t.Parallel()

	service, owner := newManagedSharedDirectService(t, readyNodeWithTaint(corev1.Taint{
		Key:    "node-cache.deckhouse.io/maintenance",
		Value:  "true",
		Effect: corev1.TaintEffectNoSchedule,
	}))
	template := podTemplateWithoutCacheMount("runtime")
	template.Spec.Tolerations = []corev1.Toleration{{
		Key:      "node-cache.deckhouse.io/maintenance",
		Operator: corev1.TolerationOpEqual,
		Value:    "true",
		Effect:   corev1.TaintEffectNoSchedule,
	}}

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if HasSchedulingGate(template) {
		t.Fatalf("did not expect scheduling gate when ready node taint is tolerated")
	}
}

func newManagedSharedDirectService(t *testing.T, objects ...client.Object) (*Service, client.Object) {
	t.Helper()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeObjects := append([]client.Object{
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	}, objects...)
	kubeClient := testkit.NewFakeClient(t, scheme, nil, kubeObjects...)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		Render: Options{RuntimeImage: "example.com/ai-models:latest"},
		ManagedCache: ManagedCacheOptions{
			Enabled: true,
			NodeSelector: map[string]string{
				"ai.deckhouse.io/node-cache":       "true",
				nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
			},
		},
		RegistrySourceNamespace:      "d8-ai-models",
		RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service, owner
}

func readyNodeWithTaint(taint corev1.Taint) *corev1.Node {
	node := readyNode()
	node.Spec.Taints = []corev1.Taint{taint}
	return node
}

func readyRuntimePodWithUsage(t *testing.T, nodeName string, summary nodecache.RuntimeUsageSummary) *corev1.Pod {
	t.Helper()

	value, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal(summary) error = %v", err)
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ai-models-node-cache-runtime-" + nodeName,
			Namespace: "d8-ai-models",
			Labels: map[string]string{
				k8snodecacheruntime.ManagedLabelKey: k8snodecacheruntime.ManagedLabelValue,
			},
			Annotations: map[string]string{
				k8snodecacheruntime.NodeNameAnnotationKey:  nodeName,
				k8snodecacheruntime.UsageSummaryAnnotation: string(value),
			},
		},
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		}}},
	}
}
