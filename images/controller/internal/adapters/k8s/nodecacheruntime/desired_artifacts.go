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
	"context"
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const podNodeNameFieldSelector = "spec.nodeName"

type DesiredArtifactsClient struct {
	client kubernetes.Interface
}

func NewDesiredArtifactsClient(client kubernetes.Interface) (*DesiredArtifactsClient, error) {
	if client == nil {
		return nil, errors.New("node cache runtime desired artifacts client must not be nil")
	}
	return &DesiredArtifactsClient{client: client}, nil
}

func NewInClusterDesiredArtifactsClient() (*DesiredArtifactsClient, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewDesiredArtifactsClient(clientset)
}

func (c *DesiredArtifactsClient) LoadNodeDesiredArtifacts(ctx context.Context, nodeName string) ([]nodecache.DesiredArtifact, error) {
	if c == nil {
		return nil, errors.New("node cache runtime desired artifacts client must not be nil")
	}

	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return nil, errors.New("node cache runtime node name must not be empty")
	}

	podList, err := c.client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(podNodeNameFieldSelector, nodeName).String(),
	})
	if err != nil {
		return nil, err
	}

	artifacts := make([]nodecache.DesiredArtifact, 0, len(podList.Items))
	for index := range podList.Items {
		pod := &podList.Items[index]
		if strings.TrimSpace(pod.Spec.NodeName) != nodeName || !IsActiveScheduledPod(pod) {
			continue
		}
		artifact, found, err := DesiredArtifactFromPod(pod)
		if err != nil {
			return nil, err
		}
		if found {
			artifacts = append(artifacts, artifact)
		}
	}

	return nodecache.NormalizeDesiredArtifacts(artifacts)
}

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

func DesiredArtifactFromPod(pod *corev1.Pod) (nodecache.DesiredArtifact, bool, error) {
	if pod == nil {
		return nodecache.DesiredArtifact{}, false, errors.New("node cache runtime pod must not be nil")
	}
	annotations := pod.GetAnnotations()
	deliveryMode := strings.TrimSpace(annotations[modeldelivery.ResolvedDeliveryModeAnnotation])
	if deliveryMode != string(modeldelivery.DeliveryModeSharedDirect) {
		return nodecache.DesiredArtifact{}, false, nil
	}
	digest := strings.TrimSpace(annotations[modeldelivery.ResolvedDigestAnnotation])
	artifactURI := strings.TrimSpace(annotations[modeldelivery.ResolvedArtifactURIAnnotation])
	if digest == "" || artifactURI == "" {
		return nodecache.DesiredArtifact{}, false, errors.New("managed pod shared-direct artifact annotations are incomplete")
	}
	artifacts, err := nodecache.NormalizeDesiredArtifacts([]nodecache.DesiredArtifact{{
		ArtifactURI: artifactURI,
		Digest:      digest,
		Family:      strings.TrimSpace(annotations[modeldelivery.ResolvedArtifactFamilyAnnotation]),
	}})
	if err != nil {
		return nodecache.DesiredArtifact{}, false, err
	}
	return artifacts[0], true, nil
}
