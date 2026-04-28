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
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceKeepsSchedulingGateWhenReadyNodeDoesNotMatchWorkloadSelector(t *testing.T) {
	t.Parallel()

	service, owner := newManagedSharedDirectService(t, readyNode())
	template := podTemplateWithoutCacheMount("runtime")
	template.Spec.NodeSelector = map[string]string{"node.deckhouse.io/pool": "gpu"}

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err != nil {
		t.Fatalf("ApplyToPodTemplate() error = %v", err)
	}
	if !HasSchedulingGate(template) {
		t.Fatalf("expected scheduling gate while ready node does not match workload selector")
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
