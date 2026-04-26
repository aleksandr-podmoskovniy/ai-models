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

import "testing"

func TestParseReferencesPreservesLegacySingleAnnotations(t *testing.T) {
	t.Parallel()

	refs, found, err := parseReferences(map[string]string{ModelAnnotation: "gemma"})
	if err != nil {
		t.Fatalf("parseReferences(model) error = %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected one model reference, got %#v", refs)
	}
	ref := refs[0]
	if !found || ref.Alias != defaultModelAlias || ref.Scope != ReferenceScopeModel || ref.Name != "gemma" {
		t.Fatalf("unexpected model reference %#v found=%v", ref, found)
	}

	refs, found, err = parseReferences(map[string]string{ClusterModelAnnotation: "gemma"})
	if err != nil {
		t.Fatalf("parseReferences(clustermodel) error = %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected one clustermodel reference, got %#v", refs)
	}
	ref = refs[0]
	if !found || ref.Alias != defaultModelAlias || ref.Scope != ReferenceScopeClusterModel || ref.Name != "gemma" {
		t.Fatalf("unexpected clustermodel reference %#v found=%v", ref, found)
	}
}

func TestParseReferenceRejectsBothScopes(t *testing.T) {
	t.Parallel()

	_, _, err := parseReferences(map[string]string{
		ModelAnnotation:        "gemma",
		ClusterModelAnnotation: "gemma",
	})
	if err == nil {
		t.Fatal("expected error when both model and clustermodel annotations are set")
	}
}

func TestParseReferencesFromModelRefs(t *testing.T) {
	t.Parallel()

	refs, found, err := parseReferences(map[string]string{
		ModelRefsAnnotation: "main=Model/gemma, embed=ClusterModel/bge-reranker",
	})
	if err != nil {
		t.Fatalf("parseReferences() error = %v", err)
	}
	if !found || len(refs) != 2 {
		t.Fatalf("unexpected refs %#v found=%v", refs, found)
	}
	if refs[0] != (Reference{Alias: "main", Scope: ReferenceScopeModel, Name: "gemma"}) {
		t.Fatalf("unexpected first ref %#v", refs[0])
	}
	if refs[1] != (Reference{Alias: "embed", Scope: ReferenceScopeClusterModel, Name: "bge-reranker"}) {
		t.Fatalf("unexpected second ref %#v", refs[1])
	}
}

func TestParseReferencesRejectsMixedLegacyAndModelRefs(t *testing.T) {
	t.Parallel()

	_, _, err := parseReferences(map[string]string{
		ModelAnnotation:     "gemma",
		ModelRefsAnnotation: "main=Model/gemma",
	})
	if err == nil {
		t.Fatal("expected mixed annotations to be rejected")
	}
}

func TestParseReferencesRejectsUnsafeAliasAndDuplicateAlias(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"Main=Model/gemma",
		"main_model=Model/gemma",
		"main=Model/gemma,main=ClusterModel/bge",
		"main=Unknown/gemma",
		"main=Model/",
	} {
		if _, _, err := parseReferences(map[string]string{ModelRefsAnnotation: value}); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
