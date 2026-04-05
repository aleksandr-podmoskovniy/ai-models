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

package catalogstatus

import (
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/controllers/publicationops"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	"github.com/deckhouse/ai-models/controller/internal/publication"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newModelReconciler(t *testing.T, objects ...client.Object) (*ModelReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t)
	kubeClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		objects...,
	)

	return &ModelReconciler{baseReconciler{
		client: kubeClient,
		options: normalizeOptions(Options{
			OperationNamespace: "d8-ai-models",
			RequeueAfter:       time.Second,
		}),
	}}, kubeClient
}

func newClusterModelReconciler(t *testing.T, objects ...client.Object) (*ClusterModelReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t)
	kubeClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		objects...,
	)

	return &ClusterModelReconciler{baseReconciler{
		client: kubeClient,
		options: normalizeOptions(Options{
			OperationNamespace: "d8-ai-models",
			RequeueAfter:       time.Second,
		}),
	}}, kubeClient
}

func testModel() *modelsv1alpha1.Model {
	return testkit.NewModel()
}

func testClusterModel() *modelsv1alpha1.ClusterModel {
	return testkit.NewClusterModel()
}

func testUploadModel() *modelsv1alpha1.Model {
	return testkit.NewUploadModel()
}

func succeededOperationForModel(t *testing.T, model *modelsv1alpha1.Model) *corev1.ConfigMap {
	t.Helper()

	operation, err := publicationops.NewConfigMap("d8-ai-models", publicationports.Request{
		Owner: publicationports.Owner{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      model.Name,
			Namespace: model.Namespace,
			UID:       model.UID,
		},
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: model.Namespace,
			Name:      model.Name,
		},
		Spec: model.Spec,
	})
	if err != nil {
		t.Fatalf("NewConfigMap() error = %v", err)
	}
	if err := publicationops.SetSucceeded(operation, publicationports.Result{
		Snapshot: publication.Snapshot{
			Identity: publication.Identity{
				Scope:     publication.ScopeNamespaced,
				Namespace: model.Namespace,
				Name:      model.Name,
			},
			Source: publication.SourceProvenance{
				Type:              modelsv1alpha1.ModelSourceTypeHuggingFace,
				ExternalReference: "deepseek-ai/DeepSeek-R1",
				ResolvedRevision:  "abc123",
			},
			Artifact: publication.PublishedArtifact{
				Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:       "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
				MediaType: "application/vnd.cncf.model.manifest.v1+json",
				SizeBytes: 42,
			},
			Resolved: publication.ResolvedProfile{
				Task:                "text-generation",
				Framework:           "transformers",
				Family:              "deepseek",
				License:             "apache-2.0",
				Architecture:        "DeepseekForCausalLM",
				Format:              "HuggingFaceCheckpoint",
				ContextWindowTokens: 8192,
				SourceRepoID:        "deepseek-ai/DeepSeek-R1",
			},
			Result: publication.Result{
				State: "Published",
				Ready: true,
			},
		},
		CleanupHandle: cleanuphandle.Handle{
			Kind: cleanuphandle.KindBackendArtifact,
			Artifact: &cleanuphandle.ArtifactSnapshot{
				Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:  "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			},
			Backend: &cleanuphandle.BackendArtifactHandle{
				Reference: "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			},
		},
	}); err != nil {
		t.Fatalf("SetSucceeded() error = %v", err)
	}

	return operation
}
