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

package workloaddelivery

import (
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentReconcilerKeepsGateWhenSharedDirectReadyNodeDoesNotMatchWorkloadSelector(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeploymentWithoutCacheMount(map[string]string{ModelAnnotation: model.Name}, 1)
	workload.Spec.Template.Spec.NodeSelector = map[string]string{"node.deckhouse.io/pool": "gpu"}
	reconciler, kubeClient := newDeploymentReconcilerWithOptions(t, modeldelivery.ServiceOptions{
		Render: modeldelivery.Options{
			RuntimeImage: "example.com/ai-models/controller-runtime:dev",
		},
		ManagedCache: modeldelivery.ManagedCacheOptions{
			Enabled: true,
			NodeSelector: map[string]string{
				"ai.deckhouse.io/node-cache":       "true",
				nodecache.RuntimeReadyNodeLabelKey: nodecache.RuntimeReadyNodeLabelValue,
			},
		},
		RegistrySourceNamespace:      testRegistryNamespace,
		RegistrySourceAuthSecretName: testRegistryAuthName,
		RuntimeImagePullSecretName:   testRuntimePullSecret,
	}, model, workload, readyNodeCacheRuntimeNode(), testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if !modeldelivery.HasSchedulingGate(&updated.Spec.Template) {
		t.Fatalf("expected scheduling gate while no ready node-cache node matches workload selector")
	}
}
