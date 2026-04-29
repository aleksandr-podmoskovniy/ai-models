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
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceRejectsStatefulSetClaimTemplateBridgeTopology(t *testing.T) {
	t.Parallel()

	service, owner := newTopologyService(t)
	template := podTemplateWithCacheMount("runtime", "model-cache", DefaultCacheMountPath)

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{
			ReplicaCount: 3,
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "model-cache"},
			}},
		},
	}, template)
	if err == nil || !strings.Contains(err.Error(), "does not support explicit cache claim template") {
		t.Fatalf("expected claim template bridge rejection, got %v", err)
	}
}

func TestServiceRejectsSharedWorkloadPVCBridgeTopology(t *testing.T) {
	t.Parallel()

	service, owner := newTopologyService(t)
	template := podTemplateWithPVCMount("runtime", "model-cache", "shared-model-cache", DefaultCacheMountPath)

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 3},
	}, template)
	if err == nil || !strings.Contains(err.Error(), "does not support explicit cache persistentVolumeClaim") {
		t.Fatalf("expected PVC bridge rejection, got %v", err)
	}
}

func TestServiceRejectsEphemeralBridgeTopology(t *testing.T) {
	t.Parallel()

	service, owner := newTopologyService(t)
	template := podTemplateWithCacheMount("runtime", "model-cache", DefaultCacheMountPath)

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err == nil || !strings.Contains(err.Error(), "does not support explicit cache volume") {
		t.Fatalf("expected ephemeral bridge rejection, got %v", err)
	}
}

func TestServiceRejectsNegativeReplicaCount(t *testing.T) {
	t.Parallel()

	service, owner := newTopologyService(t)
	template := podTemplateWithCacheMount("runtime", "model-cache", DefaultCacheMountPath)

	_, err := service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: -1},
	}, template)
	if err == nil || err.Error() != "runtime delivery replica count must not be negative" {
		t.Fatalf("unexpected error %v", err)
	}
}

func newTopologyService(t *testing.T) (*Service, client.Object) {
	t.Helper()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil, owner)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service, owner
}
