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

package uploadsession

import (
	"context"
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceGetOrCreateCreatesOwnedUploadSessionResources(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")
	owner.Name = "deepseek-r1-upload"
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner).
		Build()

	service, err := NewService(kubeClient, scheme, testUploadOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	request := testUploadOperationContext()
	request.Request.Owner.UID = types.UID("1111-2222")
	request.Request.Owner.Name = "deepseek-r1-upload"
	request.Request.Identity.Name = "deepseek-r1-upload"

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected upload session resources to be created")
	}
	if handle == nil || handle.WorkerName == "" {
		t.Fatalf("expected upload session handle, got %#v", handle)
	}
	if !strings.Contains(handle.UploadStatus.Command, "port-forward service/") {
		t.Fatalf("unexpected upload command %q", handle.UploadStatus.Command)
	}
	if !strings.Contains(handle.UploadStatus.Command, "X-AI-MODELS-FILENAME") {
		t.Fatalf("expected upload filename header in command %q", handle.UploadStatus.Command)
	}
	if handle.UploadStatus.ExpiresAt == nil {
		t.Fatal("expected upload session expiration")
	}

	serviceName, err := resourcenames.UploadSessionServiceName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionServiceName() error = %v", err)
	}
	secretName, err := resourcenames.UploadSessionSecretName(request.Request.Owner.UID)
	if err != nil {
		t.Fatalf("UploadSessionSecretName() error = %v", err)
	}

	for _, object := range []client.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: handle.WorkerName, Namespace: "d8-ai-models"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: "d8-ai-models"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "d8-ai-models"}},
	} {
		stored := object.DeepCopyObject().(client.Object)
		if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(object), stored); err != nil {
			t.Fatalf("Get(%T) error = %v", object, err)
		}
		if len(stored.GetOwnerReferences()) != 0 {
			t.Fatalf("expected no cross-namespace owner references on %T", object)
		}
	}

	pod := &corev1.Pod{}
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: handle.WorkerName, Namespace: "d8-ai-models"}, pod); err != nil {
		t.Fatalf("Get(pod) error = %v", err)
	}
	if got, want := pod.Annotations[resourcenames.OwnerNameAnnotationKey], owner.Name; got != want {
		t.Fatalf("unexpected owner-name annotation %q", got)
	}
	if got, want := pod.Annotations[resourcenames.OwnerNamespaceAnnotationKey], owner.Namespace; got != want {
		t.Fatalf("unexpected owner-namespace annotation %q", got)
	}
}

func TestBuildPodUsesUploadSessionRuntime(t *testing.T) {
	t.Parallel()

	request := testUploadOperationContext()
	request.Request.Owner.UID = types.UID("1111-2222")
	request.Request.Owner.Name = "deepseek-r1-upload"
	request.Request.Identity.Name = "deepseek-r1-upload"
	request.Request.Spec.Source.Upload.ExpectedSizeBytes = ptrTo[int64](128)

	options := testUploadOptions()
	options.OCIRegistryCASecretName = "registry-ca"

	pod, err := BuildPod(request, options, "ai-model-upload-auth-1111-2222")
	if err != nil {
		t.Fatalf("BuildPod() error = %v", err)
	}

	if got, want := pod.Spec.Containers[0].Args[0], "upload-session"; got != want {
		t.Fatalf("unexpected subcommand %q", got)
	}
	if !containsArg(pod.Spec.Containers[0].Args, "--expected-size-bytes", "128") {
		t.Fatalf("expected size arg in %#v", pod.Spec.Containers[0].Args)
	}
	if !containsArg(pod.Spec.Containers[0].Args, "--task", "text-generation") {
		t.Fatalf("expected task arg in %#v", pod.Spec.Containers[0].Args)
	}
	if !hasEnv(pod.Spec.Containers[0].Env, "AI_MODELS_OCI_CA_FILE", "/etc/ai-models/registry-ca/ca.crt") {
		t.Fatalf("expected registry CA env in %#v", pod.Spec.Containers[0].Env)
	}
	if !hasVolume(pod.Spec.Volumes, "registry-ca") {
		t.Fatalf("expected registry CA volume in %#v", pod.Spec.Volumes)
	}
}

func containsArg(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func hasEnv(env []corev1.EnvVar, name, value string) bool {
	for _, item := range env {
		if item.Name == name && item.Value == value {
			return true
		}
	}
	return false
}

func hasVolume(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}
