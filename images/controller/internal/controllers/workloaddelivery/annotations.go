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
	"strings"
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

func parseReference(annotations map[string]string) (Reference, bool, error) {
	modelName := strings.TrimSpace(annotations[ModelAnnotation])
	clusterModelName := strings.TrimSpace(annotations[ClusterModelAnnotation])
	switch {
	case modelName != "" && clusterModelName != "":
		return Reference{}, false, errors.New("workload delivery annotations must not set both model and clustermodel")
	case modelName != "":
		return Reference{Scope: ReferenceScopeModel, Name: modelName}, true, nil
	case clusterModelName != "":
		return Reference{Scope: ReferenceScopeClusterModel, Name: clusterModelName}, true, nil
	default:
		return Reference{}, false, nil
	}
}
