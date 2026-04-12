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

package catalogcleanup

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildCleanupJob(t *testing.T) {
	t.Parallel()

	job, err := buildCleanupJob(cleanupJobOwner{
		UID:       types.UID("1111-2222"),
		Kind:      "Model",
		Name:      "deepseek-r1",
		Namespace: "team-a",
	}, cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Artifact: &cleanuphandle.ArtifactSnapshot{
			Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:  "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef",
		},
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef",
		},
	}, CleanupJobOptions{
		Namespace:             "d8-ai-models",
		Image:                 "backend:latest",
		ImagePullSecretName:   "ai-models-module-registry",
		ServiceAccountName:    "ai-models-controller",
		OCIRegistrySecretName: "ai-models-dmcr-auth-write",
		Env: []corev1.EnvVar{
			{Name: "AWS_REGION", Value: "us-east-1"},
		},
	})
	if err != nil {
		t.Fatalf("buildCleanupJob() error = %v", err)
	}

	if got, want := job.Namespace, "d8-ai-models"; got != want {
		t.Fatalf("unexpected job namespace %q", got)
	}
	if got, want := job.Spec.Template.Spec.Containers[0].Args[0], "artifact-cleanup"; got != want {
		t.Fatalf("unexpected cleanup subcommand %q", got)
	}
	if got, want := job.Labels[resourcenames.AppNameLabelKey], cleanupJobAppName; got != want {
		t.Fatalf("unexpected app label %q", got)
	}
	if got, want := job.Labels[resourcenames.OwnerNamespaceLabelKey], "team-a"; got != want {
		t.Fatalf("unexpected owner namespace label %q", got)
	}
	if len(job.Spec.Template.Spec.ImagePullSecrets) != 1 || job.Spec.Template.Spec.ImagePullSecrets[0].Name != "ai-models-module-registry" {
		t.Fatalf("unexpected imagePullSecrets %#v", job.Spec.Template.Spec.ImagePullSecrets)
	}
}
