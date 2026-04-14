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

func TestParseReference(t *testing.T) {
	t.Parallel()

	ref, found, err := parseReference(map[string]string{ModelAnnotation: "gemma"})
	if err != nil {
		t.Fatalf("parseReference(model) error = %v", err)
	}
	if !found || ref.Scope != ReferenceScopeModel || ref.Name != "gemma" {
		t.Fatalf("unexpected model reference %#v found=%v", ref, found)
	}

	ref, found, err = parseReference(map[string]string{ClusterModelAnnotation: "gemma"})
	if err != nil {
		t.Fatalf("parseReference(clustermodel) error = %v", err)
	}
	if !found || ref.Scope != ReferenceScopeClusterModel || ref.Name != "gemma" {
		t.Fatalf("unexpected clustermodel reference %#v found=%v", ref, found)
	}
}

func TestParseReferenceRejectsBothScopes(t *testing.T) {
	t.Parallel()

	_, _, err := parseReference(map[string]string{
		ModelAnnotation:        "gemma",
		ClusterModelAnnotation: "gemma",
	})
	if err == nil {
		t.Fatal("expected error when both model and clustermodel annotations are set")
	}
}
