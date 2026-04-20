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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
)

func TestRenderBuildsMaterializerAgainstExistingCacheMount(t *testing.T) {
	t.Parallel()

	rendered, err := Render(Input{
		Artifact: publication.PublishedArtifact{
			Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:       "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/model@sha256:deadbeef",
			Digest:    "sha256:deadbeef",
			MediaType: "application/vnd.cncf.model.manifest.v1+json",
		},
		ArtifactFamily: "hf-safetensors-v1",
		RegistryAccess: ociregistry.Projection{
			AuthSecretName: "projected-registry-auth",
			CASecretName:   "projected-registry-ca",
		},
		CacheMount: CacheMount{
			VolumeName: "model-cache",
			MountPath:  DefaultCacheMountPath,
		},
	}, Options{
		RuntimeImage: "example.com/ai-models:latest",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got, want := rendered.InitContainer.Name, DefaultInitContainerName; got != want {
		t.Fatalf("init container name = %q, want %q", got, want)
	}
	if got, want := rendered.InitContainer.Args, []string{"materialize-artifact"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("unexpected init args %#v", got)
	}
	if got, want := rendered.ModelPath, nodecache.WorkloadModelPath(DefaultCacheMountPath); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
	if got, want := rendered.InitContainer.VolumeMounts[0].Name, "model-cache"; got != want {
		t.Fatalf("cache volume name = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, ModelPathEnv), nodecache.WorkloadModelPath(DefaultCacheMountPath); got != want {
		t.Fatalf("runtime model path env = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, ModelDigestEnv), "sha256:deadbeef"; got != want {
		t.Fatalf("runtime model digest env = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, ModelFamilyEnv), "hf-safetensors-v1"; got != want {
		t.Fatalf("runtime model family env = %q, want %q", got, want)
	}
	if got := envValue(rendered.InitContainer.Env, "AI_MODELS_MATERIALIZE_CACHE_ROOT"); got != DefaultCacheMountPath {
		t.Fatalf("cache root env = %q, want %q", got, DefaultCacheMountPath)
	}
	if got := envValue(rendered.InitContainer.Env, LogLevelEnv); got != defaultLogLevel {
		t.Fatalf("log level env = %q, want %q", got, defaultLogLevel)
	}
	if got := envValue(rendered.InitContainer.Env, "AI_MODELS_OCI_CA_FILE"); got != ociregistry.CAFilePath {
		t.Fatalf("unexpected OCI CA env %q", got)
	}
	if got := len(rendered.Volumes); got != 1 {
		t.Fatalf("expected only CA volume injection, got %#v", rendered.Volumes)
	}
}

func TestRenderBuildsDigestScopedModelPathForSharedTopology(t *testing.T) {
	t.Parallel()

	rendered, err := Render(Input{
		Artifact: publication.PublishedArtifact{
			Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:       "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/model@sha256:deadbeef",
			Digest:    "sha256:deadbeef",
			MediaType: "application/vnd.cncf.model.manifest.v1+json",
		},
		RegistryAccess: ociregistry.Projection{
			AuthSecretName: "projected-registry-auth",
		},
		CacheMount: CacheMount{
			VolumeName: "model-cache",
			MountPath:  DefaultCacheMountPath,
		},
		TopologyKind: CacheTopologySharedDirect,
	}, Options{
		RuntimeImage: "example.com/ai-models:latest",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got, want := rendered.ModelPath, nodecache.SharedArtifactModelPath(DefaultCacheMountPath, "sha256:deadbeef"); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, ModelPathEnv), nodecache.SharedArtifactModelPath(DefaultCacheMountPath, "sha256:deadbeef"); got != want {
		t.Fatalf("runtime model path env = %q, want %q", got, want)
	}
	if got := envValue(rendered.InitContainer.Env, "AI_MODELS_MATERIALIZE_SHARED_STORE"); got != "true" {
		t.Fatalf("shared store env = %q, want true", got)
	}
}

func TestRenderBuildsSharedCacheCoordinationEnv(t *testing.T) {
	t.Parallel()

	rendered, err := Render(Input{
		Artifact: publication.PublishedArtifact{
			Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:       "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/model@sha256:deadbeef",
			Digest:    "sha256:deadbeef",
			MediaType: "application/vnd.cncf.model.manifest.v1+json",
		},
		RegistryAccess: ociregistry.Projection{
			AuthSecretName: "projected-registry-auth",
		},
		CacheMount: CacheMount{
			VolumeName: "model-cache",
			MountPath:  DefaultCacheMountPath,
		},
		TopologyKind: CacheTopologySharedDirect,
		Coordination: Coordination{
			Mode: CoordinationModeShared,
		},
	}, Options{
		RuntimeImage: "example.com/ai-models:latest",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got, want := envValue(rendered.InitContainer.Env, "AI_MODELS_MATERIALIZE_COORDINATION_MODE"), CoordinationModeShared; got != want {
		t.Fatalf("coordination mode = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, ModelPathEnv), nodecache.SharedArtifactModelPath(DefaultCacheMountPath, "sha256:deadbeef"); got != want {
		t.Fatalf("runtime model path env = %q, want %q", got, want)
	}
	if got := envValue(rendered.InitContainer.Env, "AI_MODELS_MATERIALIZE_COORDINATION_NAMESPACE"); got != "" {
		t.Fatalf("did not expect coordination namespace env, got %q", got)
	}

	var holderEnv *corev1.EnvVar
	for i := range rendered.InitContainer.Env {
		if rendered.InitContainer.Env[i].Name == "AI_MODELS_MATERIALIZE_COORDINATION_HOLDER_ID" {
			holderEnv = &rendered.InitContainer.Env[i]
			break
		}
	}
	if holderEnv == nil || holderEnv.ValueFrom == nil || holderEnv.ValueFrom.FieldRef == nil || holderEnv.ValueFrom.FieldRef.FieldPath != "metadata.name" {
		t.Fatalf("expected holder id downward API env, got %#v", holderEnv)
	}
}

func TestRenderRejectsMissingCacheVolume(t *testing.T) {
	t.Parallel()

	_, err := Render(Input{
		Artifact: publication.PublishedArtifact{
			Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:    "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/model@sha256:deadbeef",
			Digest: "sha256:deadbeef",
		},
		RegistryAccess: ociregistry.Projection{AuthSecretName: "projected-registry-auth"},
	}, Options{
		RuntimeImage: "example.com/ai-models:latest",
	})
	if err == nil || err.Error() != "runtime delivery cache volume name must not be empty" {
		t.Fatalf("expected missing cache volume error, got %v", err)
	}
}

func TestRenderRejectsMismatchedCacheMountContract(t *testing.T) {
	t.Parallel()

	_, err := Render(Input{
		Artifact: publication.PublishedArtifact{
			Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:    "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/model@sha256:deadbeef",
			Digest: "sha256:deadbeef",
		},
		RegistryAccess: ociregistry.Projection{AuthSecretName: "projected-registry-auth"},
		CacheMount: CacheMount{
			VolumeName: "model-cache",
			MountPath:  "/models",
		},
	}, Options{
		RuntimeImage: "example.com/ai-models:latest",
	})
	if err == nil || err.Error() != "runtime delivery cache mount contract is inconsistent" {
		t.Fatalf("expected inconsistent cache mount error, got %v", err)
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
