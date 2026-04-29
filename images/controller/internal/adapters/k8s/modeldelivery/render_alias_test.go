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
	"encoding/json"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestRenderBuildsAliasRuntimeContractForMultipleSharedDirectModels(t *testing.T) {
	t.Parallel()

	rendered, err := Render(Input{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []BindingInput{
			{Alias: "main", Artifact: publishedArtifactWithDigest("sha256:primary"), ArtifactFamily: "hf-safetensors-v1"},
			{Alias: "embed", Artifact: publishedArtifactWithDigest("sha256:embed"), ArtifactFamily: "embedding-v1"},
		},
		CacheMount: CacheMount{
			VolumeName: DefaultManagedCacheName,
			MountPath:  DefaultCacheMountPath,
		},
		TopologyKind: CacheTopologyDirect,
	}, Options{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got, want := envValue(rendered.RuntimeEnv, ModelPathEnv), nodecache.WorkloadModelAliasPath(DefaultCacheMountPath, "main"); got != want {
		t.Fatalf("primary model path = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, ModelsDirEnv), nodecache.WorkloadModelsDirPath(DefaultCacheMountPath); got != want {
		t.Fatalf("models dir = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, NamedModelPathEnv("embed")), nodecache.WorkloadModelAliasPath(DefaultCacheMountPath, "embed"); got != want {
		t.Fatalf("named model path = %q, want %q", got, want)
	}
	if got, want := envValue(rendered.RuntimeEnv, NamedModelDigestEnv("embed")), "sha256:embed"; got != want {
		t.Fatalf("named model digest = %q, want %q", got, want)
	}
	if rendered.ResolvedModels == "" || envValue(rendered.RuntimeEnv, ModelsEnv) == "" {
		t.Fatalf("expected rendered model manifest in annotation and env")
	}

	var runtimeEntries []map[string]string
	if err := json.Unmarshal([]byte(envValue(rendered.RuntimeEnv, ModelsEnv)), &runtimeEntries); err != nil {
		t.Fatalf("decode runtime models env: %v", err)
	}
	if _, leaksURI := runtimeEntries[0]["uri"]; leaksURI {
		t.Fatalf("runtime models env must not expose internal artifact URI: %#v", runtimeEntries[0])
	}
	var resolvedEntries []map[string]any
	if err := json.Unmarshal([]byte(rendered.ResolvedModels), &resolvedEntries); err != nil {
		t.Fatalf("decode resolved models annotation: %v", err)
	}
	if got := resolvedEntries[0]["uri"]; got == "" {
		t.Fatalf("resolved models annotation must keep artifact URI for node-cache runtime")
	}
	if got := resolvedEntries[0]["sizeBytes"]; got == nil {
		t.Fatalf("resolved models annotation must keep artifact size for node-cache admission")
	}
}

func TestRenderBuildsAliasMountsForManagedSharedDirectVolumes(t *testing.T) {
	t.Parallel()

	rendered, err := Render(Input{
		Artifact: publishedArtifactWithDigest("sha256:primary"),
		Bindings: []BindingInput{
			{Alias: "main", Artifact: publishedArtifactWithDigest("sha256:primary")},
			{Alias: "draft", Artifact: publishedArtifactWithDigest("sha256:draft")},
		},
		CacheMount: CacheMount{
			VolumeName: DefaultManagedCacheName,
			MountPath:  DefaultCacheMountPath,
		},
		TopologyKind: CacheTopologyDirect,
	}, Options{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := len(rendered.Volumes); got != 0 {
		t.Fatalf("render must keep CSI volume ownership in service layer, got %d", got)
	}
	if got, want := rendered.RuntimeVolumeMounts[0].Name, managedModelVolumeName(DefaultManagedCacheName, "main"); got != want {
		t.Fatalf("main mount volume = %q, want %q", got, want)
	}
	if got, want := rendered.RuntimeVolumeMounts[1].MountPath, nodecache.WorkloadModelAliasPath(DefaultCacheMountPath, "draft"); got != want {
		t.Fatalf("draft mount path = %q, want %q", got, want)
	}
	if !rendered.RuntimeVolumeMounts[1].ReadOnly {
		t.Fatalf("expected read-only shared-direct model mount")
	}
}

func publishedArtifactWithDigest(digest string) publication.PublishedArtifact {
	return publication.PublishedArtifact{
		Kind:      modelsv1alpha1.ModelArtifactLocationKindOCI,
		URI:       "dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/model@" + digest,
		Digest:    digest,
		MediaType: "application/vnd.cncf.model.manifest.v1+json",
		SizeBytes: 42,
	}
}
