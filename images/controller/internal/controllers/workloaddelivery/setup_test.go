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
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestEnabledWorkloadKindsSkipsRayClusterWhenKubeRayCRDsAreMissing(t *testing.T) {
	t.Parallel()

	kinds := enabledWorkloadKinds(meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}}))
	if got, want := len(kinds), len(coreWorkloadKinds); got != want {
		t.Fatalf("kind count = %d, want %d", got, want)
	}
	for _, kind := range kinds {
		if kind.kind == "RayCluster" {
			t.Fatalf("RayCluster should not be enabled without discovery mapping")
		}
	}
}

func TestEnabledWorkloadKindsAddsRayClusterWhenKubeRayCRDsExist(t *testing.T) {
	t.Parallel()

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{rayServiceGVK.GroupVersion()})
	mapper.Add(rayServiceGVK, meta.RESTScopeNamespace)
	mapper.Add(rayClusterGVK, meta.RESTScopeNamespace)

	kinds := enabledWorkloadKinds(mapper)
	if got, want := len(kinds), len(coreWorkloadKinds)+1; got != want {
		t.Fatalf("kind count = %d, want %d", got, want)
	}
	if got, want := kinds[len(kinds)-1].kind, "RayCluster"; got != want {
		t.Fatalf("last kind = %q, want %q", got, want)
	}
}
