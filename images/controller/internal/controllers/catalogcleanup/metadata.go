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

package catalogcleanup

import (
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	garbageCollectionRequestAppName = "ai-models-dmcr-gc-request"
)

func garbageCollectionRequestLabels(owner cleanupOwner) map[string]string {
	labels := resourcenames.OwnerLabels(garbageCollectionRequestAppName, owner.Kind, owner.Name, owner.UID, owner.Namespace)
	labels[dmcrGCRequestLabelKey] = dmcrGCRequestLabelValue
	return labels
}

func garbageCollectionRequestKey(namespace string, ownerUID types.UID) client.ObjectKey {
	return client.ObjectKey{
		Namespace: namespace,
		Name:      dmcrGCRequestSecretName(ownerUID),
	}
}

func mergeLabels(existing, desired map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(desired))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range desired {
		merged[key] = value
	}
	return merged
}
