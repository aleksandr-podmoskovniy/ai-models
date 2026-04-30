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
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	ModelAnnotation        = "ai.deckhouse.io/model"
	ClusterModelAnnotation = "ai.deckhouse.io/clustermodel"
)

type ReferenceScope string

const (
	ReferenceScopeModel        ReferenceScope = "Model"
	ReferenceScopeClusterModel ReferenceScope = "ClusterModel"
)

type Reference struct {
	Scope ReferenceScope
	Name  string
}

func parseReferences(annotations map[string]string) ([]Reference, bool, error) {
	seen := map[string]struct{}{}
	refs, err := parseReferenceList(annotations[ModelAnnotation], ReferenceScopeModel, seen)
	if err != nil {
		return nil, false, err
	}
	clusterRefs, err := parseReferenceList(annotations[ClusterModelAnnotation], ReferenceScopeClusterModel, seen)
	if err != nil {
		return nil, false, err
	}
	refs = append(refs, clusterRefs...)
	if len(refs) == 0 {
		return nil, false, nil
	}
	return refs, true, nil
}

func parseReferenceList(value string, scope ReferenceScope, seen map[string]struct{}) ([]Reference, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	refs := make([]Reference, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, errors.New("workload delivery model list must not contain empty names")
		}
		if strings.ContainsAny(name, "=/") {
			return nil, fmt.Errorf("workload delivery model name %q must not contain renaming syntax, scopes or slashes", name)
		}
		if err := validateReferenceName(name); err != nil {
			return nil, fmt.Errorf("invalid workload delivery model name %q: %w", name, err)
		}
		if _, found := seen[name]; found {
			return nil, fmt.Errorf("duplicate workload delivery model name %q", name)
		}
		seen[name] = struct{}{}
		refs = append(refs, Reference{Scope: scope, Name: name})
	}
	if len(refs) == 0 {
		return nil, nil
	}
	return refs, nil
}

func validateReferenceName(name string) error {
	if name = strings.TrimSpace(name); name == "" {
		return errors.New("model name must not be empty")
	}
	if problems := validation.IsDNS1123Subdomain(name); len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
