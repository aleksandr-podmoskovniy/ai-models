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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(request *publicationports.Request)
		wantErr bool
	}{
		{name: "accepted upload"},
		{
			name: "missing owner uid fails closed",
			mutate: func(request *publicationports.Request) {
				request.Owner.UID = types.UID("")
			},
			wantErr: true,
		},
		{
			name: "owner identity mismatch fails closed",
			mutate: func(request *publicationports.Request) {
				request.Owner.Namespace = "team-b"
			},
			wantErr: true,
		},
		{
			name: "non-upload source is rejected",
			mutate: func(request *publicationports.Request) {
				request.Spec.Source = modelsv1alpha1.ModelSourceSpec{
					URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
				}
			},
			wantErr: true,
		},
		{
			name: "missing upload source is rejected",
			mutate: func(request *publicationports.Request) {
				request.Spec.Source = modelsv1alpha1.ModelSourceSpec{}
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := testUploadRequest()
			if tc.mutate != nil {
				tc.mutate(&request)
			}

			err := validateRequest(request)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateRequest() error = %v", err)
			}
		})
	}
}

func TestServiceGetOrCreateRejectsInvalidRequestBeforeSecretWrite(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(owner).
		Build()

	service, err := NewService(kubeClient, scheme, testUploadOptions())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	request := testUploadRequest()
	request.Owner.UID = owner.UID
	request.Owner.Name = owner.Name
	request.Identity.Name = owner.Name
	request.Spec.Source = modelsv1alpha1.ModelSourceSpec{
		URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
	}

	handle, created, err := service.GetOrCreate(context.Background(), owner, request)
	if err == nil {
		t.Fatal("expected invalid upload-session request to fail")
	}
	if created || handle != nil {
		t.Fatalf("unexpected handle=%#v created=%t", handle, created)
	}

	var secrets corev1.SecretList
	if err := kubeClient.List(context.Background(), &secrets, client.InNamespace(testUploadOptions().Runtime.Namespace)); err != nil {
		t.Fatalf("List(secrets) error = %v", err)
	}
	if len(secrets.Items) != 0 {
		t.Fatalf("expected invalid request to avoid secret writes, got %#v", secrets.Items)
	}
}
