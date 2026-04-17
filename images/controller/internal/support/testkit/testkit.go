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

package testkit

import (
	"testing"

	apiinstall "github.com/deckhouse/ai-models/api/core/install"
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type SchemeInstaller func(*runtime.Scheme) error

func NewScheme(t *testing.T, installers ...SchemeInstaller) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	apiinstall.Install(scheme)
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) error = %v", err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(networkingv1) error = %v", err)
	}
	for _, install := range installers {
		if err := install(scheme); err != nil {
			t.Fatalf("AddToScheme() error = %v", err)
		}
	}

	return scheme
}

func NewFakeClient(
	t *testing.T,
	scheme *runtime.Scheme,
	statusSubresources []client.Object,
	objects ...client.Object,
) client.Client {
	t.Helper()

	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(statusSubresources) > 0 {
		builder = builder.WithStatusSubresource(statusSubresources...)
	}
	if len(objects) > 0 {
		builder = builder.WithObjects(objects...)
	}

	return builder.Build()
}

func HuggingFaceSpec() modelsv1alpha1.ModelSpec {
	return modelsv1alpha1.ModelSpec{
		Source: modelsv1alpha1.ModelSourceSpec{
			URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1?revision=main",
		},
	}
}

func UploadSpec() modelsv1alpha1.ModelSpec {
	return modelsv1alpha1.ModelSpec{
		Source: modelsv1alpha1.ModelSourceSpec{
			Upload: &modelsv1alpha1.UploadModelSource{},
		},
	}
}

func NewModel() *modelsv1alpha1.Model {
	return &modelsv1alpha1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deepseek-r1",
			Namespace: "team-a",
			UID:       types.UID("550e8400-e29b-41d4-a716-446655440000"),
		},
		Spec: HuggingFaceSpec(),
	}
}

func NewUploadModel() *modelsv1alpha1.Model {
	return &modelsv1alpha1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deepseek-r1-upload",
			Namespace: "team-a",
			UID:       types.UID("550e8400-e29b-41d4-a716-446655440111"),
		},
		Spec: UploadSpec(),
	}
}

func NewClusterModel() *modelsv1alpha1.ClusterModel {
	return &modelsv1alpha1.ClusterModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deepseek-r1",
			UID:  types.UID("11111111-2222-3333-4444-555555555555"),
		},
		Spec: HuggingFaceSpec(),
	}
}

func NewOCIRegistryWriteAuthSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"registry.internal.local":{"username":"ai-models","password":"secret"}}}`),
			"username":          []byte("ai-models"),
			"password":          []byte("secret"),
		},
	}
}

func NewOCIRegistryCASecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt": []byte("-----BEGIN CERTIFICATE-----\nTEST\n-----END CERTIFICATE-----\n"),
		},
	}
}
