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

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
)

const (
	ModelAnnotation        = "ai.deckhouse.io/model"
	ClusterModelAnnotation = "ai.deckhouse.io/clustermodel"
	ModelRefsAnnotation    = "ai.deckhouse.io/model-refs"

	defaultModelAlias = "model"
)

type ReferenceScope string

const (
	ReferenceScopeModel        ReferenceScope = "Model"
	ReferenceScopeClusterModel ReferenceScope = "ClusterModel"
)

type Reference struct {
	Alias string
	Scope ReferenceScope
	Name  string
}

func parseReferences(annotations map[string]string) ([]Reference, bool, error) {
	modelRefs := strings.TrimSpace(annotations[ModelRefsAnnotation])
	modelName := strings.TrimSpace(annotations[ModelAnnotation])
	clusterModelName := strings.TrimSpace(annotations[ClusterModelAnnotation])

	if modelRefs != "" {
		if modelName != "" || clusterModelName != "" {
			return nil, false, errors.New("workload delivery annotations must not mix model-refs with model or clustermodel")
		}
		refs, err := parseModelRefs(modelRefs)
		return refs, len(refs) > 0, err
	}

	switch {
	case modelName != "" && clusterModelName != "":
		return nil, false, errors.New("workload delivery annotations must not set both model and clustermodel")
	case modelName != "":
		return []Reference{{Alias: defaultModelAlias, Scope: ReferenceScopeModel, Name: modelName}}, true, nil
	case clusterModelName != "":
		return []Reference{{Alias: defaultModelAlias, Scope: ReferenceScopeClusterModel, Name: clusterModelName}}, true, nil
	default:
		return nil, false, nil
	}
}

func usesModelRefsAnnotation(annotations map[string]string) bool {
	return strings.TrimSpace(annotations[ModelRefsAnnotation]) != ""
}

func parseModelRefs(value string) ([]Reference, error) {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	if len(parts) == 0 {
		return nil, errors.New("model-refs annotation must contain at least one reference")
	}
	seenAliases := make(map[string]struct{}, len(parts))
	refs := make([]Reference, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ref, err := parseModelRef(part)
		if err != nil {
			return nil, err
		}
		if _, found := seenAliases[ref.Alias]; found {
			return nil, fmt.Errorf("duplicate model alias %q", ref.Alias)
		}
		seenAliases[ref.Alias] = struct{}{}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil, errors.New("model-refs annotation must contain at least one reference")
	}
	return refs, nil
}

func parseModelRef(value string) (Reference, error) {
	alias, target, found := strings.Cut(value, "=")
	if !found {
		return Reference{}, fmt.Errorf("model reference %q must use alias=Kind/name", value)
	}
	alias = strings.TrimSpace(alias)
	if err := nodecache.ValidateModelAlias(alias); err != nil {
		return Reference{}, fmt.Errorf("invalid model alias %q: %w", alias, err)
	}
	kind, name, found := strings.Cut(strings.TrimSpace(target), "/")
	if !found {
		return Reference{}, fmt.Errorf("model reference %q must use alias=Kind/name", value)
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") {
		return Reference{}, fmt.Errorf("model reference %q must contain a single non-empty name", value)
	}
	switch {
	case strings.EqualFold(strings.TrimSpace(kind), string(ReferenceScopeModel)):
		return Reference{Alias: alias, Scope: ReferenceScopeModel, Name: name}, nil
	case strings.EqualFold(strings.TrimSpace(kind), string(ReferenceScopeClusterModel)):
		return Reference{Alias: alias, Scope: ReferenceScopeClusterModel, Name: name}, nil
	default:
		return Reference{}, fmt.Errorf("model reference %q has unsupported kind %q", value, kind)
	}
}
