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

package modelobject

import (
	"fmt"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func KindFor(object client.Object) (string, error) {
	switch object.(type) {
	case *modelsv1alpha1.Model:
		return modelsv1alpha1.ModelKind, nil
	case *modelsv1alpha1.ClusterModel:
		return modelsv1alpha1.ClusterModelKind, nil
	default:
		return "", fmt.Errorf("unsupported model object type %T", object)
	}
}

func GetStatus(object client.Object) (modelsv1alpha1.ModelStatus, error) {
	switch typed := object.(type) {
	case *modelsv1alpha1.Model:
		return typed.Status, nil
	case *modelsv1alpha1.ClusterModel:
		return typed.Status, nil
	default:
		return modelsv1alpha1.ModelStatus{}, fmt.Errorf("unsupported model object type %T", object)
	}
}

func SetStatus(object client.Object, status modelsv1alpha1.ModelStatus) error {
	switch typed := object.(type) {
	case *modelsv1alpha1.Model:
		typed.Status = status
		return nil
	case *modelsv1alpha1.ClusterModel:
		typed.Status = status
		return nil
	default:
		return fmt.Errorf("unsupported model object type %T", object)
	}
}

func PublicationRequest(object client.Object, spec modelsv1alpha1.ModelSpec) (publicationports.Request, error) {
	kind, err := KindFor(object)
	if err != nil {
		return publicationports.Request{}, err
	}

	switch object.(type) {
	case *modelsv1alpha1.Model:
		return publicationports.Request{
			Owner: publicationports.Owner{
				Kind:      kind,
				Name:      object.GetName(),
				Namespace: object.GetNamespace(),
				UID:       object.GetUID(),
			},
			Identity: publication.Identity{
				Scope:     publication.ScopeNamespaced,
				Namespace: object.GetNamespace(),
				Name:      object.GetName(),
			},
			Spec: spec,
		}, nil
	case *modelsv1alpha1.ClusterModel:
		return publicationports.Request{
			Owner: publicationports.Owner{
				Kind: kind,
				Name: object.GetName(),
				UID:  object.GetUID(),
			},
			Identity: publication.Identity{
				Scope: publication.ScopeCluster,
				Name:  object.GetName(),
			},
			Spec: spec,
		}, nil
	default:
		return publicationports.Request{}, fmt.Errorf("unsupported model object type %T", object)
	}
}
