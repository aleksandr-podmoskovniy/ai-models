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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceGetOrCreateCreatesOwnedUploadSessionResources(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	service, err := NewService(kubeClient, scheme, testUploadOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	operation := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ai-model-publication-1111-2222",
			Namespace: "d8-ai-models",
			UID:       types.UID("operation-uid"),
		},
	}

	request := testUploadOperationContext()
	request.Request.Owner.UID = types.UID("1111-2222")
	request.Request.Owner.Name = "deepseek-r1-upload"
	request.Request.Identity.Name = "deepseek-r1-upload"
	request.OperationName = operation.Name
	request.OperationNamespace = operation.Namespace

	handle, created, err := service.GetOrCreate(context.Background(), operation, request)
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
		if len(stored.GetOwnerReferences()) != 1 {
			t.Fatalf("expected controller owner references on %T", object)
		}
	}
}

func TestBuildPodUsesUploadSessionRuntime(t *testing.T) {
	t.Parallel()

	request := testUploadOperationContext()
	request.Request.Owner.UID = types.UID("1111-2222")
	request.Request.Owner.Name = "deepseek-r1-upload"
	request.Request.Identity.Name = "deepseek-r1-upload"
	request.OperationName = "ai-model-publication-1111-2222"
	request.Request.Spec.Source.Upload.ExpectedSizeBytes = ptrTo[int64](128)

	options := testUploadOptions()
	options.OCIRegistryCASecretName = "registry-ca"

	pod, err := BuildPod(request, options, "ai-model-upload-auth-1111-2222")
	if err != nil {
		t.Fatalf("BuildPod() error = %v", err)
	}

	if got, want := pod.Spec.Containers[0].Command[0], "ai-models-backend-upload-session"; got != want {
		t.Fatalf("unexpected command %q", got)
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
