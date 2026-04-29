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

package workloaddelivery

import (
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentReconcilerAppliesMultipleModelRefs(t *testing.T) {
	t.Parallel()

	primary := readyModel()
	secondary := readyModel()
	secondary.Name = "embed"
	secondaryDigest := "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	secondary.Status.Artifact.URI = "registry.internal.local/ai-models/catalog/namespaced/team-a/embed@" + secondaryDigest
	secondary.Status.Artifact.Digest = secondaryDigest
	secondary.Status.Resolved.Family = "embedding"
	workload := annotatedDeployment(map[string]string{
		ModelRefsAnnotation: "main=Model/gemma,embed=Model/embed",
	}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, primary, secondary, workload, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName))

	result := reconcileDeployment(t, reconciler, workload)
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected reconcile result %#v", result)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(deployment) error = %v", err)
	}
	if got, want := len(updated.Spec.Template.Spec.InitContainers), 2; got != want {
		t.Fatalf("init container count = %d, want %d", got, want)
	}
	if !hasInitContainer(updated.Spec.Template.Spec.InitContainers, "ai-models-materializer-main") {
		t.Fatalf("expected main model materializer")
	}
	if !hasInitContainer(updated.Spec.Template.Spec.InitContainers, "ai-models-materializer-embed") {
		t.Fatalf("expected embed model materializer")
	}
	if got, want := len(updated.Spec.Template.Spec.ImagePullSecrets), 1; got != want {
		t.Fatalf("image pull secret count = %d, want %d", got, want)
	}
	runtimeImagePullSecretName, err := resourcenames.RuntimeImagePullSecretName(workload.UID)
	if err != nil {
		t.Fatalf("RuntimeImagePullSecretName() error = %v", err)
	}
	if got, want := updated.Spec.Template.Spec.ImagePullSecrets[0].Name, runtimeImagePullSecretName; got != want {
		t.Fatalf("image pull secret name = %q, want %q", got, want)
	}
	if got, want := runtimeEnvValue(updated.Spec.Template.Spec.Containers, modeldelivery.ModelPathEnv), nodecache.WorkloadModelAliasPath(modeldelivery.DefaultCacheMountPath, "main"); got != want {
		t.Fatalf("primary model path env = %q, want %q", got, want)
	}
	if got, want := runtimeEnvValue(updated.Spec.Template.Spec.Containers, modeldelivery.NamedModelPathEnv("embed")), nodecache.WorkloadModelAliasPath(modeldelivery.DefaultCacheMountPath, "embed"); got != want {
		t.Fatalf("embed model path env = %q, want %q", got, want)
	}
	if got := updated.Spec.Template.Annotations[modeldelivery.ResolvedModelsAnnotation]; got == "" {
		t.Fatalf("expected resolved models annotation")
	}
}
