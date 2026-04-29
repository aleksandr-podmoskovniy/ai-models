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
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func TestRequestValidateRejectsInvalidBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*publicationports.Request)
		wantErr string
	}{
		{
			name: "missing owner kind",
			mutate: func(request *publicationports.Request) {
				request.Owner.Kind = ""
			},
			wantErr: "owner kind",
		},
		{
			name: "invalid identity",
			mutate: func(request *publicationports.Request) {
				request.Identity.Scope = publication.Scope("broken")
			},
			wantErr: "unsupported publication scope",
		},
		{
			name: "missing huggingface source",
			mutate: func(request *publicationports.Request) {
				request.Spec.Source.URL = ""
			},
			wantErr: "source.url or source.upload",
		},
		{
			name: "cluster scoped huggingface auth secret is unsupported",
			mutate: func(request *publicationports.Request) {
				request.Owner.Namespace = ""
				request.Identity.Scope = publication.ScopeCluster
				request.Identity.Namespace = ""
				request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "hf-auth"}
			},
			wantErr: "authSecretRef is not supported for cluster-scoped owners",
		},
		{
			name: "namespaced huggingface auth secret rejects foreign namespace",
			mutate: func(request *publicationports.Request) {
				request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{
					Namespace: "other-team",
					Name:      "hf-auth",
				}
			},
			wantErr: "must match owner namespace",
		},
		{
			name: "upload rejected",
			mutate: func(request *publicationports.Request) {
				request.Spec.Source.URL = ""
				request.Spec.Source.Upload = &modelsv1alpha1.UploadModelSource{}
			},
			wantErr: "requires a staged upload handle",
		},
		{
			name: "ollama source is fail closed until adapter exists",
			mutate: func(request *publicationports.Request) {
				request.Spec.Source.URL = "https://ollama.com/library/qwen3.6"
			},
			wantErr: "ollama source publication is not implemented yet",
		},
		{
			name: "ollama auth secret is rejected until auth contract exists",
			mutate: func(request *publicationports.Request) {
				request.Spec.Source.URL = "https://ollama.com/library/qwen3.6"
				request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "ollama-auth"}
			},
			wantErr: "ollama authSecretRef is not supported yet",
		},
		{
			name: "unsupported non huggingface host",
			mutate: func(request *publicationports.Request) {
				request.Spec.Source.URL = "https://example.invalid/model"
			},
			wantErr: "unsupported source URL host",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := testOperationRequest()
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
		{
			name: "missing publish concurrency limit",
			mutate: func(options *Options) {
				options.MaxConcurrentWorkers = 0
			},
			wantErr: "max concurrent workers",
		},
		{
			name: "unsupported source fetch mode",
			mutate: func(options *Options) {
				options.SourceFetch = "broken"
			},
			wantErr: "source fetch mode",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			options := testOptions()
			tc.mutate(&options)

			err := validateOptions(SourceWorkerPlan{}, options)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if got := err.Error(); got == "" || !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestSourcePlanBuildsStagedUploadPlan(t *testing.T) {
	t.Parallel()

	request := testOperationRequest()
	request.Spec.Source = modelsv1alpha1.ModelSourceSpec{Upload: &modelsv1alpha1.UploadModelSource{}}
	request.UploadStage = &cleanuphandle.UploadStagingHandle{
		Bucket:   "ai-models",
		Key:      "raw/1111-2222/model.gguf",
		FileName: "model.gguf",
	}

	got, err := sourcePlan(request)
	if err != nil {
		t.Fatalf("sourcePlan() error = %v", err)
	}
	if got.SourceType != modelsv1alpha1.ModelSourceTypeUpload || got.Upload == nil {
		t.Fatalf("unexpected source plan %#v", got)
	}
	if got.Upload.Stage != *request.UploadStage {
		t.Fatalf("upload stage = %#v, want %#v", got.Upload.Stage, *request.UploadStage)
	}
}

func TestPreflightRejectsOwnerIdentityMismatch(t *testing.T) {
	t.Parallel()

	request := testOperationRequest()
	request.Owner.Name = "other-model"

	var service Service
	if err := service.preflight(request, SourceWorkerPlan{}); err == nil {
		t.Fatal("expected owner binding error")
	}
}

func TestOptionsValidateDirectHuggingFaceDoesNotRequireObjectStorage(t *testing.T) {
	t.Parallel()

	options := testOptions()
	options.SourceFetch = publicationports.SourceFetchModeDirect
	options.ObjectStorage = storageprojection.Options{}

	plan := SourceWorkerPlan{
		SourceType: modelsv1alpha1.ModelSourceTypeHuggingFace,
		HuggingFace: &HuggingFaceSourcePlan{
			RepoID: "owner/model",
		},
	}

	if err := validateOptions(plan, options); err != nil {
		t.Fatalf("validateOptions() error = %v", err)
	}
}

func TestOptionsValidateMirrorHuggingFaceRequiresObjectStorage(t *testing.T) {
	t.Parallel()

	options := testOptions()
	options.SourceFetch = publicationports.SourceFetchModeMirror
	options.ObjectStorage = storageprojection.Options{}

	plan := SourceWorkerPlan{
		SourceType: modelsv1alpha1.ModelSourceTypeHuggingFace,
		HuggingFace: &HuggingFaceSourcePlan{
			RepoID: "owner/model",
		},
	}

	err := validateOptions(plan, options)
	if err == nil {
		t.Fatal("expected object storage validation error")
	}
	if got, want := err.Error(), "object storage"; !strings.Contains(got, want) {
		t.Fatalf("validateOptions() error = %q, want substring %q", got, want)
	}
}

func TestOptionsValidateRequiresUploadEndpoint(t *testing.T) {
	t.Parallel()

	options := testOptions()
	options.SourceFetch = publicationports.SourceFetchModeDirect
	options.OCIDirectUploadEndpoint = ""
	options.ObjectStorage = storageprojection.Options{
		CASecretName: "ai-models-artifacts-ca",
	}

	plan := SourceWorkerPlan{
		SourceType: modelsv1alpha1.ModelSourceTypeHuggingFace,
		HuggingFace: &HuggingFaceSourcePlan{
			RepoID: "owner/model",
		},
	}

	err := validateOptions(plan, options)
	if err == nil {
		t.Fatal("expected OCI direct upload endpoint validation error")
	}
	if got, want := err.Error(), "OCI direct upload endpoint"; !strings.Contains(got, want) {
		t.Fatalf("validateOptions() error = %q, want substring %q", got, want)
	}
}
