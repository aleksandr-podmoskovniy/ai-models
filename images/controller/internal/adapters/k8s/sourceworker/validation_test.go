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

package sourceworker

import (
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestRequestValidateRejectsInvalidBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*publicationports.OperationContext)
		wantErr string
	}{
		{
			name: "missing owner kind",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Owner.Kind = ""
			},
			wantErr: "owner kind",
		},
		{
			name: "invalid identity",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Identity.Scope = publication.Scope("broken")
			},
			wantErr: "unsupported publication scope",
		},
		{
			name: "missing huggingface source",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Spec.Source.URL = ""
			},
			wantErr: "source.url or source.upload",
		},
		{
			name: "cluster scoped huggingface auth secret without namespace",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Owner.Namespace = ""
				request.Request.Identity.Scope = publication.ScopeCluster
				request.Request.Identity.Namespace = ""
				request.Request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "hf-auth"}
			},
			wantErr: "authSecretRef namespace",
		},
		{
			name: "http missing url",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Spec.Source.URL = "https://example.invalid/model.tgz"
				request.Request.Spec.Source.URL = ""
				request.Request.Spec.RuntimeHints = &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"}
			},
			wantErr: "source.url or source.upload",
		},
		{
			name: "cluster scoped http auth secret without namespace",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Owner.Namespace = ""
				request.Request.Identity.Scope = publication.ScopeCluster
				request.Request.Identity.Namespace = ""
				request.Request.Spec.Source.URL = "https://example.invalid/model.tgz"
				request.Request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "http-auth"}
				request.Request.Spec.RuntimeHints = &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"}
			},
			wantErr: "authSecretRef namespace",
		},
		{
			name: "namespaced http auth secret rejects foreign namespace",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Spec.Source.URL = "https://example.invalid/model.tgz"
				request.Request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{
					Namespace: "other-team",
					Name:      "http-auth",
				}
				request.Request.Spec.RuntimeHints = &modelsv1alpha1.ModelRuntimeHints{Task: "text-generation"}
			},
			wantErr: "must match owner namespace",
		},
		{
			name: "http missing task",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Spec.Source.URL = "https://example.invalid/model.tgz"
				request.Request.Spec.RuntimeHints = nil
			},
			wantErr: "runtimeHints.task",
		},
		{
			name: "upload rejected",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Spec.Source.URL = ""
				request.Request.Spec.Source.Upload = &modelsv1alpha1.UploadModelSource{}
			},
			wantErr: "must be implemented as a session",
		},
		{
			name: "unsupported source scheme",
			mutate: func(request *publicationports.OperationContext) {
				request.Request.Spec.Source.URL = "oci://example.invalid/model"
			},
			wantErr: "unsupported source URL scheme",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := testOperationContext()
			tc.mutate(&request)

			_, err := sourcePlan(request)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if got := err.Error(); got == "" || !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestOptionsValidateRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*Options)
		wantErr string
	}{
		{
			name: "missing namespace",
			mutate: func(options *Options) {
				options.Namespace = ""
			},
			wantErr: "namespace",
		},
		{
			name: "missing image",
			mutate: func(options *Options) {
				options.Image = ""
			},
			wantErr: "image",
		},
		{
			name: "missing service account",
			mutate: func(options *Options) {
				options.ServiceAccountName = ""
			},
			wantErr: "serviceAccountName",
		},
		{
			name: "missing repository prefix",
			mutate: func(options *Options) {
				options.OCIRepositoryPrefix = ""
			},
			wantErr: "OCI repository prefix",
		},
		{
			name: "missing registry secret",
			mutate: func(options *Options) {
				options.OCIRegistrySecretName = ""
			},
			wantErr: "OCI registry secret name",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			options := testOptions()
			tc.mutate(&options)

			err := validateOptions(options)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if got := err.Error(); got == "" || !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}
