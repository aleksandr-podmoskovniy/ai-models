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

package modeldelivery

import (
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
)

func TestServiceRejectsMissingCacheMount(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "runtime"}},
		},
	}

	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err == nil || err.Error() != "runtime delivery annotated workload must mount writable model cache at \"/data/modelcache\"" {
		t.Fatalf("unexpected error %v", err)
	}
	if !IsWorkloadContractError(err) {
		t.Fatalf("expected workload contract error, got %T", err)
	}
}

func TestServiceRejectsAmbiguousCacheMountVolume(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewModel()
	kubeClient := testkit.NewFakeClient(t, scheme, nil,
		owner,
		testkit.NewOCIRegistryWriteAuthSecret("d8-ai-models", "ai-models-dmcr-auth-read"),
	)

	service, err := NewService(kubeClient, scheme, ServiceOptions{
		RegistrySourceNamespace: "d8-ai-models",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	template := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "runtime-a",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "cache-a",
						MountPath: DefaultCacheMountPath,
					}},
				},
				{
					Name: "runtime-b",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "cache-b",
						MountPath: DefaultCacheMountPath,
					}},
				},
			},
		},
	}

	_, err = service.ApplyToPodTemplate(context.Background(), owner, ApplyRequest{
		Artifact: publishedArtifact(),
		Topology: TopologyHints{ReplicaCount: 1},
	}, template)
	if err == nil || err.Error() != "runtime delivery cache mount \"/data/modelcache\" must reference a single backing volume" {
		t.Fatalf("unexpected error %v", err)
	}
	if !IsWorkloadContractError(err) {
		t.Fatalf("expected workload contract error, got %T", err)
	}
}
