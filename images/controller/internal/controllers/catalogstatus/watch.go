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

package catalogstatus

import (
	"context"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func mapPodToModelRequests(_ context.Context, object client.Object) []reconcile.Request {
	name, namespace, ok := ownerReferenceForModel(object)
	if !ok {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name, Namespace: namespace}}}
}

func mapPodToClusterModelRequests(_ context.Context, object client.Object) []reconcile.Request {
	name, ok := ownerReferenceForClusterModel(object)
	if !ok {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
}

func ownerReferenceForModel(object client.Object) (string, string, bool) {
	kind, name, namespace := ownerReferenceFromMetadata(object)
	if kind != modelsv1alpha1.ModelKind || name == "" || namespace == "" {
		return "", "", false
	}
	return name, namespace, true
}

func ownerReferenceForClusterModel(object client.Object) (string, bool) {
	kind, name, namespace := ownerReferenceFromMetadata(object)
	if kind != modelsv1alpha1.ClusterModelKind || name == "" || namespace != "" {
		return "", false
	}
	return name, true
}

func ownerReferenceFromMetadata(object client.Object) (string, string, string) {
	if object == nil {
		return "", "", ""
	}

	annotations := object.GetAnnotations()
	kind := strings.TrimSpace(annotations[resourcenames.OwnerKindAnnotationKey])
	name := strings.TrimSpace(annotations[resourcenames.OwnerNameAnnotationKey])
	namespace := strings.TrimSpace(annotations[resourcenames.OwnerNamespaceAnnotationKey])
	if kind != "" && name != "" {
		return kind, name, namespace
	}

	labels := object.GetLabels()
	return strings.TrimSpace(labels[resourcenames.OwnerKindLabelKey]),
		strings.TrimSpace(labels[resourcenames.OwnerNameLabelKey]),
		strings.TrimSpace(labels[resourcenames.OwnerNamespaceLabelKey])
}
