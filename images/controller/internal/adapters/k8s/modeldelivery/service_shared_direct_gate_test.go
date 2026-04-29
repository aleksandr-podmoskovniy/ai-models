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

	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
)

func TestServiceKeepsSchedulingGateWhenManagedCacheCannotFitArtifact(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil, owner)
	service, err := NewService(kubeClient, scheme, ServiceOptions{
		ManagedCache: ManagedCacheOptions{
			Enabled:       true,
			CapacityBytes: 10,
		},
		DeliveryAuthKey:         testDeliveryAuthKey,
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
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
