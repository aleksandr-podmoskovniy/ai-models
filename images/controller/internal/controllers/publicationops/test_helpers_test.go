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

package publicationops

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	"github.com/deckhouse/ai-models/controller/internal/publication"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PhasePending   = publicationports.PhasePending
	PhaseRunning   = publicationports.PhaseRunning
	PhaseSucceeded = publicationports.PhaseSucceeded
	PhaseFailed    = publicationports.PhaseFailed
)

func baseRequest(name string, uid types.UID) publicationports.Request {
	return publicationports.Request{
		Owner: publicationports.Owner{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      name,
			Namespace: "team-a",
			UID:       uid,
		},
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: "team-a",
			Name:      name,
		},
		Spec: modelsv1alpha1.ModelSpec{
			Source: modelsv1alpha1.ModelSourceSpec{
				Type: modelsv1alpha1.ModelSourceTypeHuggingFace,
				HuggingFace: &modelsv1alpha1.HuggingFaceModelSource{
					RepoID: "deepseek-ai/DeepSeek-R1",
				},
			},
			RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{
				Task: "text-generation",
			},
		},
	}
}

func newPublicationOperationReconciler(t *testing.T, scheme *runtime.Scheme, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	kubeClient := testkit.NewFakeClient(t, scheme, nil, objects...)

	reconciler, err := newReconciler(kubeClient, scheme, Options{
		PublishPod: PublishPodOptions{
			Namespace:             "d8-ai-models",
			Image:                 "backend:latest",
			ServiceAccountName:    "ai-models-controller",
			OCIRepositoryPrefix:   "registry.internal.local/ai-models",
			OCIRegistrySecretName: "ai-models-publication-registry",
		},
	})
	if err != nil {
		t.Fatalf("newReconciler() error = %v", err)
	}

	return reconciler, kubeClient
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	return testkit.NewScheme(t)
}

func huggingFaceRequest() publicationports.Request {
	request := baseRequest("deepseek-r1", types.UID("1111-2222"))
	request.Spec.Source.HuggingFace.Revision = "main"
	return request
}

func httpRequest() publicationports.Request {
	request := baseRequest("deepseek-r1-http", types.UID("1111-2223"))
	request.Spec.Source = modelsv1alpha1.ModelSourceSpec{
		Type: modelsv1alpha1.ModelSourceTypeHTTP,
		HTTP: &modelsv1alpha1.HTTPModelSource{
			URL: "https://downloads.example/models/deepseek-r1.tar.gz",
		},
	}
	return request
}

func uploadRequest(expectedFormat modelsv1alpha1.ModelUploadFormat) publicationports.Request {
	request := baseRequest("deepseek-r1-upload", types.UID("1111-2224"))
	request.Spec.Source = modelsv1alpha1.ModelSourceSpec{
		Type: modelsv1alpha1.ModelSourceTypeUpload,
		Upload: &modelsv1alpha1.UploadModelSource{
			ExpectedFormat: expectedFormat,
		},
	}
	return request
}

func sampleResult() publicationports.Result {
	return publicationports.Result{
		Snapshot: publication.Snapshot{
			Identity: publication.Identity{
				Scope: publication.ScopeCluster,
				Name:  "deepseek-r1",
			},
			Source: publication.SourceProvenance{
				Type: modelsv1alpha1.ModelSourceTypeHuggingFace,
			},
			Artifact: publication.PublishedArtifact{
				Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:  "registry.internal.local/ai-models/catalog/cluster/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
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
				URI:  "registry.internal.local/ai-models/catalog/cluster/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			},
			Backend: &cleanuphandle.BackendArtifactHandle{
				Reference: "registry.internal.local/ai-models/catalog/cluster/deepseek-r1/550e8400-e29b-41d4-a716-446655440000@sha256:deadbeef",
			},
		},
	}
}

func sampleWorkerResultJSON() string {
	return `{"source":{"type":"HuggingFace","externalReference":"deepseek-ai/DeepSeek-R1","resolvedRevision":"abc123"},"artifact":{"kind":"OCI","uri":"registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/1111-2222@sha256:deadbeef","digest":"sha256:deadbeef","mediaType":"application/vnd.cncf.model.manifest.v1+json","sizeBytes":123},"resolved":{"task":"text-generation","framework":"transformers","family":"deepseek","license":"apache-2.0","architecture":"DeepseekForCausalLM","format":"HuggingFaceCheckpoint","contextWindowTokens":8192,"sourceRepoID":"deepseek-ai/DeepSeek-R1"},"cleanupHandle":{"kind":"BackendArtifact","artifact":{"kind":"OCI","uri":"registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/1111-2222@sha256:deadbeef","digest":"sha256:deadbeef"},"backend":{"reference":"registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1/1111-2222@sha256:deadbeef"}}}`
}

func metav1ObjectMeta(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	}
}

func mustNewOperation(t *testing.T, request publicationports.Request) *corev1.ConfigMap {
	t.Helper()

	operation, err := NewConfigMap("d8-ai-models", request)
	if err != nil {
		t.Fatalf("NewConfigMap() error = %v", err)
	}

	return operation
}

func mustSetRunning(t *testing.T, operation *corev1.ConfigMap, workerName string) {
	t.Helper()

	if err := SetRunning(operation, workerName); err != nil {
		t.Fatalf("SetRunning() error = %v", err)
	}
}

func mustReconcile(t *testing.T, reconciler *Reconciler, operation client.Object) ctrl.Result {
	t.Helper()

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(operation),
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	return result
}

func mustGetConfigMap(t *testing.T, kubeClient client.Client, operation client.Object) corev1.ConfigMap {
	t.Helper()

	var updated corev1.ConfigMap
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(operation), &updated); err != nil {
		t.Fatalf("Get(operation) error = %v", err)
	}

	return updated
}
