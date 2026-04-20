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

package nodecacheruntime

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	options := Options{
		Enabled:            true,
		Namespace:          "d8-ai-models",
		RuntimeImage:       "runtime:latest",
		ServiceAccountName: "ai-models-node-cache-runtime",
		StorageClassName:   "ai-models-node-cache",
		SharedVolumeSize:   "64Gi",
		MaxTotalSize:       "200Gi",
		MaxUnusedAge:       "24h",
		ScanInterval:       "5m",
		OCIAuthSecretName:  "ai-models-dmcr-auth-read",
		NodeSelectorLabels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"},
	}
	if err := options.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestOptionsValidateRejectsInvalidSharedVolumeSize(t *testing.T) {
	t.Parallel()

	options := Options{
		Enabled:            true,
		Namespace:          "d8-ai-models",
		RuntimeImage:       "runtime:latest",
		ServiceAccountName: "ai-models-node-cache-runtime",
		StorageClassName:   "ai-models-node-cache",
		SharedVolumeSize:   "invalid",
		MaxTotalSize:       "200Gi",
		MaxUnusedAge:       "24h",
		ScanInterval:       "5m",
		OCIAuthSecretName:  "ai-models-dmcr-auth-read",
		NodeSelectorLabels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"},
	}
	if err := options.Validate(); err == nil {
		t.Fatal("expected invalid shared volume size error")
	}
}

func TestOptionsMatchesNode(t *testing.T) {
	t.Parallel()

	options := Options{NodeSelectorLabels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a", Labels: map[string]string{"node-role.deckhouse.io/ai-models-cache": "enabled"}}}
	if !options.MatchesNode(node) {
		t.Fatal("expected node selector match")
	}
}

func TestOptionsMatchesNodeRejectsMissingLabel(t *testing.T) {
	t.Parallel()

	options := Options{NodeSelectorLabels: map[string]string{"node-role.deckhouse.io/ai-models-cache": ""}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"}}
	if options.MatchesNode(node) {
		t.Fatal("expected missing label to fail matchLabels semantics")
	}
}
