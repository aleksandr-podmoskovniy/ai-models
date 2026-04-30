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
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
)

func TestRenderBuildsSharedDirectWithoutMaterializer(t *testing.T) {
	t.Parallel()

	rendered, err := Render(Input{
		Artifact:       renderArtifact(),
		ArtifactFamily: "hf-safetensors-v1",
		Bindings: []BindingInput{{
			Name:           "gemma",
			Artifact:       renderArtifact(),
			ArtifactFamily: "hf-safetensors-v1",
		}},
		CacheMount: CacheMount{
			VolumeName: DefaultManagedCacheName,
			MountPath:  DefaultCacheMountPath,
		},
		TopologyKind:              CacheTopologyDirect,
		LegacyImagePullSecretName: "legacy-runtime-pull",
	}, Options{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got, want := rendered.ImagePullSecretNamesPrune, []string{"legacy-runtime-pull"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("image pull secret prune list = %#v, want %#v", got, want)
	}
	if got, want := rendered.ModelPath, nodecache.WorkloadNamedModelPath(DefaultCacheMountPath, "gemma"); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, ModelsDirEnv), nodecache.WorkloadModelsDirPath(DefaultCacheMountPath); got != want {
		t.Fatalf("runtime models dir env = %q, want %q", got, want)
	}
	if got := envValue(rendered.RuntimeEnv, legacyModelPathEnv); got != "" {
		t.Fatalf("did not expect legacy runtime model path env, got %q", got)
	}
	if len(rendered.Volumes) != 0 {
		t.Fatalf("did not expect registry CA volumes for shared-direct render, got %#v", rendered.Volumes)
	}
}

func TestRenderRejectsNonDirectTopology(t *testing.T) {
	t.Parallel()

	_, err := Render(Input{
		Artifact: renderArtifact(),
		Bindings: []BindingInput{{
			Name:     "gemma",
			Artifact: renderArtifact(),
		}},
		CacheMount: CacheMount{
			VolumeName: DefaultManagedCacheName,
			MountPath:  DefaultCacheMountPath,
		},
		TopologyKind: CacheTopologyPerPod,
	}, Options{})
	if err == nil || !strings.Contains(err.Error(), "supports only SharedDirect or SharedPVC delivery") {
		t.Fatalf("expected non-direct delivery error, got %v", err)
	}
}

func TestRenderBuildsSharedPVCMount(t *testing.T) {
	t.Parallel()

	rendered, err := Render(Input{
		Artifact: renderArtifact(),
		Bindings: []BindingInput{{
			Name:     "gemma",
			Artifact: renderArtifact(),
		}},
		CacheMount: CacheMount{
			VolumeName: DefaultSharedPVCVolumeName,
			MountPath:  DefaultCacheMountPath,
		},
		SharedPVCClaimName: "ai-models-cache-abc123",
		TopologyKind:       CacheTopologySharedPVC,
	}, Options{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got, want := len(rendered.Volumes), 1; got != want {
		t.Fatalf("volumes = %d, want %d", got, want)
	}
	claim := rendered.Volumes[0].PersistentVolumeClaim
	if claim == nil || claim.ClaimName != "ai-models-cache-abc123" || !claim.ReadOnly {
		t.Fatalf("unexpected shared pvc volume %#v", rendered.Volumes[0])
	}
	if got, want := rendered.RuntimeVolumeMounts[0].MountPath, nodecache.WorkloadModelsDirPath(DefaultCacheMountPath); got != want {
		t.Fatalf("shared pvc mount path = %q, want %q", got, want)
	}
}

func TestRenderRejectsMissingCacheVolume(t *testing.T) {
	t.Parallel()

	_, err := Render(Input{
		Artifact: renderArtifact(),
		Bindings: []BindingInput{{
			Name:     "gemma",
			Artifact: renderArtifact(),
		}},
		TopologyKind: CacheTopologyDirect,
	}, Options{})
	if err == nil || err.Error() != "runtime delivery cache volume name must not be empty" {
		t.Fatalf("expected missing cache volume error, got %v", err)
	}
}

func TestRenderRejectsMismatchedCacheMountContract(t *testing.T) {
	t.Parallel()

	_, err := Render(Input{
		Artifact: renderArtifact(),
		Bindings: []BindingInput{{
			Name:     "gemma",
			Artifact: renderArtifact(),
		}},
		CacheMount: CacheMount{
			VolumeName: DefaultManagedCacheName,
			MountPath:  "/models",
		},
		TopologyKind: CacheTopologyDirect,
	}, Options{})
	if err == nil || err.Error() != "runtime delivery cache mount contract is inconsistent" {
		t.Fatalf("expected inconsistent cache mount error, got %v", err)
	}
}

func renderArtifact() publication.PublishedArtifact {
	return publication.PublishedArtifact{
		Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
		URI:       "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/model@sha256:deadbeef",
		Digest:    "sha256:deadbeef",
		MediaType: "application/vnd.cncf.model.manifest.v1+json",
	}
}

func envValue(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}
