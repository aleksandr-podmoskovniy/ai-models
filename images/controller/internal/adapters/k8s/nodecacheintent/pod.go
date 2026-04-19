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
	"errors"
	"strings"

	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	corev1 "k8s.io/api/core/v1"
)

func IsActiveScheduledPod(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if strings.TrimSpace(pod.Spec.NodeName) == "" {
		return false
	}
	if pod.DeletionTimestamp != nil {
		return false
	}
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return false
	default:
		return true
	}
}

func IntentFromPod(pod *corev1.Pod) (intentcontract.ArtifactIntent, bool, error) {
	if pod == nil {
		return intentcontract.ArtifactIntent{}, false, errors.New("node cache intent pod must not be nil")
	}
	annotations := pod.GetAnnotations()
	digest := strings.TrimSpace(annotations[modeldelivery.ResolvedDigestAnnotation])
	artifactURI := strings.TrimSpace(annotations[modeldelivery.ResolvedArtifactURIAnnotation])
	if digest == "" && artifactURI == "" {
		return intentcontract.ArtifactIntent{}, false, nil
	}
	if digest == "" || artifactURI == "" {
		return intentcontract.ArtifactIntent{}, false, errors.New("managed pod node cache intent annotations are incomplete")
	}
	intent, err := intentcontract.NormalizeIntents([]intentcontract.ArtifactIntent{{
		ArtifactURI: artifactURI,
		Digest:      digest,
		Family:      strings.TrimSpace(annotations[modeldelivery.ResolvedArtifactFamilyAnnotation]),
	}})
	if err != nil {
		return intentcontract.ArtifactIntent{}, false, err
	}
	return intent[0], true, nil
}
