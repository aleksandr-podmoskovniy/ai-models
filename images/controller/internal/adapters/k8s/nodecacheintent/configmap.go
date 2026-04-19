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

package nodecacheintent

import (
	"strings"

	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ManagedLabelKey  = "ai.deckhouse.io/node-cache-intent"
	ManagedLabelValue = "managed"
	NodeNameLabelKey = "ai.deckhouse.io/node-name"
	NodeNameAnnotationKey = "ai.deckhouse.io/node-name-full"
)

func DesiredConfigMap(namespace, nodeName string, intents []intentcontract.ArtifactIntent) (*corev1.ConfigMap, error) {
	name, err := resourcenames.NodeCacheIntentConfigMapName(nodeName)
	if err != nil {
		return nil, err
	}
	payload, err := intentcontract.EncodeIntents(intents)
	if err != nil {
		return nil, err
	}
	nodeName = strings.TrimSpace(nodeName)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: strings.TrimSpace(namespace),
			Labels: map[string]string{
				ManagedLabelKey:  ManagedLabelValue,
				NodeNameLabelKey: resourcenames.TruncateLabelValue(nodeName),
			},
			Annotations: map[string]string{
				NodeNameAnnotationKey: nodeName,
			},
		},
		Data: map[string]string{
			intentcontract.DataKey: string(payload),
		},
	}, nil
}
