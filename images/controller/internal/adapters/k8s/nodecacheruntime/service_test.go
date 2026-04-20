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

package nodecacheruntime

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceApplyCreatesPVCAndPod(t *testing.T) {
	t.Parallel()

	service, kubeClient := newTestService(t)
	owner := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a", UID: "worker-a-uid"}}

	if err := service.Apply(context.Background(), owner, testRuntimeSpec()); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	pod := &corev1.Pod{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-runtime-worker-a"}, pod); err != nil {
		t.Fatalf("Get(Pod) error = %v", err)
	}
	if len(pod.OwnerReferences) != 1 || pod.OwnerReferences[0].Name != "worker-a" {
		t.Fatalf("unexpected pod owner refs %#v", pod.OwnerReferences)
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-worker-a"}, pvc); err != nil {
		t.Fatalf("Get(PVC) error = %v", err)
	}
	if len(pvc.OwnerReferences) != 1 || pvc.OwnerReferences[0].Name != "worker-a" {
		t.Fatalf("unexpected PVC owner refs %#v", pvc.OwnerReferences)
	}
}

func TestServiceApplyDeletesDriftedPodForRecreate(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}

	driftedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ai-models-node-cache-runtime-worker-a",
			Namespace: "d8-ai-models",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-a",
			Containers: []corev1.Container{{
				Name:  DefaultContainerName,
				Image: "old:tag",
			}},
		},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(driftedPod).Build()
	service, err := NewService(kubeClient, scheme)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	owner := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a", UID: "worker-a-uid"}}
	if err := service.Apply(context.Background(), owner, testRuntimeSpec()); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	pod := &corev1.Pod{}
	err = kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: "ai-models-node-cache-runtime-worker-a"}, pod)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected drifted pod to be deleted for recreate, got err=%v", err)
	}
}

func TestServiceDeleteRemovesPodAndPVC(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}

	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ai-models-node-cache-runtime-worker-a", Namespace: "d8-ai-models"}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "ai-models-node-cache-worker-a", Namespace: "d8-ai-models"}}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod, pvc).Build()
	service, err := NewService(kubeClient, scheme)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.Delete(context.Background(), "d8-ai-models", "worker-a"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	err = kubeClient.Get(context.Background(), client.ObjectKeyFromObject(pod), &corev1.Pod{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected Pod deletion, got err=%v", err)
	}
	err = kubeClient.Get(context.Background(), client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected PVC deletion, got err=%v", err)
	}
}

func newTestService(t *testing.T) (*Service, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	service, err := NewService(kubeClient, scheme)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service, kubeClient
}

func testRuntimeSpec() RuntimeSpec {
	return RuntimeSpec{
		Namespace:          "d8-ai-models",
		NodeName:           "worker-a",
		RuntimeImage:       "runtime:latest",
		ServiceAccountName: "ai-models-node-cache-runtime",
		StorageClassName:   "ai-models-node-cache",
		SharedVolumeSize:   "64Gi",
		MaxTotalSize:       "200Gi",
		MaxUnusedAge:       "24h",
		ScanInterval:       "5m",
		OCIAuthSecretName:  "ai-models-dmcr-auth-read",
	}
}
