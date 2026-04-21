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

package nodecachesubstrate

import (
	"context"
	"log/slog"
	"testing"

	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecachesubstrate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSetupWithManagerSkipsDisabledController(t *testing.T) {
	t.Parallel()

	if err := SetupWithManager(nil, nil, Options{}); err != nil {
		t.Fatalf("SetupWithManager() error = %v", err)
	}
}

func TestSetupWithManagerRejectsInvalidEnabledOptions(t *testing.T) {
	t.Parallel()

	if err := SetupWithManager(nil, nil, Options{Enabled: true}); err == nil {
		t.Fatal("SetupWithManager() error = nil, want validation error")
	}
}

func TestReconcileDisabledPathIsNoOp(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	reconciler := &Reconciler{
		client:  fake.NewClientBuilder().WithScheme(scheme).Build(),
		logger:  slog.Default(),
		options: Options{},
	}
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
}

func TestReconcileCreatesManagedStorageObjects(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}
	managedLVG := k8sadapters.NewLVMVolumeGroup("lvg-0")
	managedLVG.SetLabels(map[string]string{k8sadapters.ManagedLabelKey: k8sadapters.ManagedLabelValue})
	managedLVG.Object["status"] = map[string]any{"phase": "Ready"}

	reconciler := &Reconciler{
		client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(node, managedLVG).
			Build(),
		logger: slog.Default(),
		options: Options{
			Enabled:               true,
			MaxSize:               "200Gi",
			StorageClassName:      "ai-models-node-cache",
			VolumeGroupSetName:    "ai-models-node-cache",
			VolumeGroupNameOnNode: "ai-models-cache",
			ThinPoolName:          "model-cache",
			NodeSelectorLabels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			BlockDeviceMatchLabels: map[string]string{
				"status.blockdevice.storage.deckhouse.io/model": "nvme",
			},
		},
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	lvgSet := k8sadapters.NewLVMVolumeGroupSet("ai-models-node-cache")
	if err := reconciler.client.Get(context.Background(), types.NamespacedName{Name: lvgSet.GetName()}, lvgSet); err != nil {
		t.Fatalf("Get(LVMVolumeGroupSet) error = %v", err)
	}
	localStorageClass := k8sadapters.NewLocalStorageClass("ai-models-node-cache")
	if err := reconciler.client.Get(context.Background(), types.NamespacedName{Name: localStorageClass.GetName()}, localStorageClass); err != nil {
		t.Fatalf("Get(LocalStorageClass) error = %v", err)
	}
}

func TestReconcileRequeuesWithoutReadyManagedLVGs(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	reconciler := &Reconciler{
		client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}).
			Build(),
		logger: slog.Default(),
		options: Options{
			Enabled:               true,
			MaxSize:               "200Gi",
			StorageClassName:      "ai-models-node-cache",
			VolumeGroupSetName:    "ai-models-node-cache",
			VolumeGroupNameOnNode: "ai-models-cache",
			ThinPoolName:          "model-cache",
			NodeSelectorLabels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			BlockDeviceMatchLabels: map[string]string{
				"status.blockdevice.storage.deckhouse.io/model": "nvme",
			},
		},
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("RequeueAfter = 0, want non-zero")
	}
}
