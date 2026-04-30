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
	"strings"
	"testing"
)

func TestParseReferencesAcceptsSingleAndMultiModelAnnotations(t *testing.T) {
	t.Parallel()

	refs, found, err := parseReferences(map[string]string{ModelAnnotation: "gemma,bge-m3"})
	if err != nil {
		t.Fatalf("parseReferences(model) error = %v", err)
	}
	if !found || len(refs) != 2 {
		t.Fatalf("expected two model references, got %#v found=%v", refs, found)
	}
	if refs[0] != (Reference{Scope: ReferenceScopeModel, Name: "gemma"}) {
		t.Fatalf("unexpected first model reference %#v", refs[0])
	}
	if refs[1] != (Reference{Scope: ReferenceScopeModel, Name: "bge-m3"}) {
		t.Fatalf("unexpected second model reference %#v", refs[1])
	}

	refs, found, err = parseReferences(map[string]string{ClusterModelAnnotation: "gemma"})
	if err != nil {
		t.Fatalf("parseReferences(clustermodel) error = %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected one clustermodel reference, got %#v", refs)
	}
	if refs[0] != (Reference{Scope: ReferenceScopeClusterModel, Name: "gemma"}) {
		t.Fatalf("unexpected clustermodel reference %#v found=%v", refs[0], found)
	}
}

func TestParseReferencesAcceptsKubernetesResourceNames(t *testing.T) {
	t.Parallel()

	longName := "model-" + strings.Repeat("a", 60) + ".v1"
	refs, found, err := parseReferences(map[string]string{ClusterModelAnnotation: longName})
	if err != nil {
		t.Fatalf("parseReferences() error = %v", err)
	}
	if !found || len(refs) != 1 || refs[0].Name != longName {
		t.Fatalf("unexpected refs %#v found=%v", refs, found)
	}
}

func TestParseReferenceRejectsDuplicateMountNamesAcrossScopes(t *testing.T) {
	t.Parallel()

	_, _, err := parseReferences(map[string]string{
		ModelAnnotation:        "gemma",
		ClusterModelAnnotation: "gemma",
	})
	if err == nil {
		t.Fatal("expected duplicate model names across scopes to be rejected")
	}
}

func TestParseReferencesAllowsBothScopesWhenNamesAreDistinct(t *testing.T) {
	t.Parallel()

	refs, found, err := parseReferences(map[string]string{
		ModelAnnotation:        "team-embed",
		ClusterModelAnnotation: "qwen3-14b",
	})
	if err != nil {
		t.Fatalf("parseReferences() error = %v", err)
	}
	if !found || len(refs) != 2 {
		t.Fatalf("unexpected refs %#v found=%v", refs, found)
	}
	if refs[0] != (Reference{Scope: ReferenceScopeModel, Name: "team-embed"}) {
		t.Fatalf("unexpected first ref %#v", refs[0])
	}
	if refs[1] != (Reference{Scope: ReferenceScopeClusterModel, Name: "qwen3-14b"}) {
		t.Fatalf("unexpected second ref %#v", refs[1])
	}
}

func TestParseReferencesRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"Main",
		"main_model",
		"main=gemma",
		"team/gemma",
		"gemma;bge",
		"gemma\nbge",
		"gemma,,bge",
		"gemma,gemma",
	} {
		if _, _, err := parseReferences(map[string]string{ModelAnnotation: value}); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
