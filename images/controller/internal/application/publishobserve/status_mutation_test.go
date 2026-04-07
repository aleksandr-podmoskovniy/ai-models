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

package publishobserve

import (
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPlanCatalogStatusMutation(t *testing.T) {
	t.Parallel()

	expiresAt := metav1.NewTime(time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC))
	snapshot := publication.Snapshot{
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: "team-a",
			Name:      "deepseek-r1",
		},
		Source: publication.SourceProvenance{
			Type: modelsv1alpha1.ModelSourceTypeHuggingFace,
		},
		Artifact: publication.PublishedArtifact{
			Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:  "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/uid@sha256:deadbeef",
		},
		Result: publication.Result{
			State: "Published",
			Ready: true,
		},
	}
	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/uid@sha256:deadbeef",
		},
	}

	cases := []struct {
		name    string
		input   CatalogStatusMutationInput
		assert  func(t *testing.T, got CatalogStatusMutationPlan)
		wantErr bool
	}{
		{
			name: "running remote observation requeues without delete",
			input: CatalogStatusMutationInput{
				Runtime: CatalogStatusRuntimeResult{
					Generation: 7,
					SourceType: modelsv1alpha1.ModelSourceTypeHuggingFace,
					Observation: publicationdomain.Observation{
						Phase: publicationdomain.OperationPhaseRunning,
					},
				},
			},
			assert: func(t *testing.T, got CatalogStatusMutationPlan) {
				t.Helper()
				if got.Status.Phase != modelsv1alpha1.ModelPhasePublishing || !got.Requeue {
					t.Fatalf("unexpected plan %#v", got)
				}
				if got.DeleteRuntime || got.DeleteRuntimeBeforePersist {
					t.Fatalf("did not expect delete flags in %#v", got)
				}
			},
		},
		{
			name: "running upload observation projects wait-for-upload",
			input: CatalogStatusMutationInput{
				Runtime: CatalogStatusRuntimeResult{
					Generation: 7,
					SourceType: modelsv1alpha1.ModelSourceTypeUpload,
					Observation: publicationdomain.Observation{
						Phase: publicationdomain.OperationPhaseRunning,
						Upload: &modelsv1alpha1.ModelUploadStatus{
							Command:    "curl -T model.tar",
							Repository: "registry.example/upload",
							ExpiresAt:  &expiresAt,
						},
					},
				},
			},
			assert: func(t *testing.T, got CatalogStatusMutationPlan) {
				t.Helper()
				if got.Status.Phase != modelsv1alpha1.ModelPhaseWaitForUpload || got.Status.Upload == nil {
					t.Fatalf("unexpected plan %#v", got)
				}
			},
		},
		{
			name: "failed terminal observation deletes runtime before persist",
			input: CatalogStatusMutationInput{
				Runtime: CatalogStatusRuntimeResult{
					Generation:    7,
					SourceType:    modelsv1alpha1.ModelSourceTypeHTTP,
					DeleteRuntime: true,
					Observation: publicationdomain.Observation{
						Phase:   publicationdomain.OperationPhaseFailed,
						Message: "download failed",
					},
				},
			},
			assert: func(t *testing.T, got CatalogStatusMutationPlan) {
				t.Helper()
				if got.Status.Phase != modelsv1alpha1.ModelPhaseFailed {
					t.Fatalf("unexpected plan %#v", got)
				}
				if !got.DeleteRuntime || !got.DeleteRuntimeBeforePersist {
					t.Fatalf("expected delete-before-persist plan %#v", got)
				}
			},
		},
		{
			name: "successful terminal observation keeps cleanup handle",
			input: CatalogStatusMutationInput{
				Runtime: CatalogStatusRuntimeResult{
					Generation:    7,
					SourceType:    modelsv1alpha1.ModelSourceTypeHuggingFace,
					DeleteRuntime: true,
					Observation: publicationdomain.Observation{
						Phase:         publicationdomain.OperationPhaseSucceeded,
						Snapshot:      &snapshot,
						CleanupHandle: &handle,
					},
				},
			},
			assert: func(t *testing.T, got CatalogStatusMutationPlan) {
				t.Helper()
				if got.Status.Phase != modelsv1alpha1.ModelPhaseReady || got.CleanupHandle == nil {
					t.Fatalf("unexpected plan %#v", got)
				}
				if !got.DeleteRuntime || got.DeleteRuntimeBeforePersist {
					t.Fatalf("unexpected delete flags %#v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := PlanCatalogStatusMutation(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("PlanCatalogStatusMutation() error = %v", err)
			}
			tc.assert(t, got)
		})
	}
}

func TestPlanFailedCatalogStatusMutation(t *testing.T) {
	t.Parallel()

	got, err := PlanFailedCatalogStatusMutation(modelsv1alpha1.ModelStatus{}, 7, modelsv1alpha1.ModelSourceTypeHTTP, "fetch failed")
	if err != nil {
		t.Fatalf("PlanFailedCatalogStatusMutation() error = %v", err)
	}
	if got.Status.Phase != modelsv1alpha1.ModelPhaseFailed {
		t.Fatalf("unexpected phase %q", got.Status.Phase)
	}
	if got.DeleteRuntime || got.DeleteRuntimeBeforePersist || got.Requeue {
		t.Fatalf("unexpected plan flags %#v", got)
	}
}
