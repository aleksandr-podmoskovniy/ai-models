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
	"encoding/json"
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	podNodeNameFieldSelector = "spec.nodeName"
	csiPodNameAttribute      = "csi.storage.k8s.io/pod.name"
	csiPodNamespaceAttribute = "csi.storage.k8s.io/pod.namespace"
	csiPodUIDAttribute       = "csi.storage.k8s.io/pod.uid"
)

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
		podArtifacts, found, err := DesiredArtifactsFromPod(pod)
		if err != nil {
			return nil, err
		}
		if found {
			artifacts = append(artifacts, podArtifacts...)
		}
	}

	return nodecache.NormalizeDesiredArtifacts(artifacts)
}

func (c *DesiredArtifactsClient) AllowCSIPublish(ctx context.Context, nodeName string, attributes map[string]string, digest string) (bool, error) {
	if c == nil {
		return false, errors.New("node cache runtime desired artifacts client must not be nil")
	}
	pod, ok, err := c.csiPublishPod(ctx, nodeName, attributes, digest)
	if err != nil || !ok {
		return false, err
	}
	artifacts, found, err := DesiredArtifactsFromPod(pod)
	if err != nil || !found {
		return false, err
	}
	return desiredArtifactsContainDigest(artifacts, digest), nil
}

func (c *DesiredArtifactsClient) csiPublishPod(ctx context.Context, nodeName string, attributes map[string]string, digest string) (*corev1.Pod, bool, error) {
	nodeName = strings.TrimSpace(nodeName)
	podName := strings.TrimSpace(attributes[csiPodNameAttribute])
	podNamespace := strings.TrimSpace(attributes[csiPodNamespaceAttribute])
	podUID := strings.TrimSpace(attributes[csiPodUIDAttribute])
	if nodeName == "" || podName == "" || podNamespace == "" || podUID == "" || strings.TrimSpace(digest) == "" {
		return nil, false, nil
	}

	pod, err := c.client.CoreV1().Pods(podNamespace).Get(ctx, podName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if string(pod.UID) != podUID || strings.TrimSpace(pod.Spec.NodeName) != nodeName || !IsActiveScheduledPod(pod) {
		return nil, false, nil
	}
	return pod, true, nil
}

func desiredArtifactsContainDigest(artifacts []nodecache.DesiredArtifact, digest string) bool {
	digest = strings.TrimSpace(digest)
	for _, artifact := range artifacts {
		if artifact.Digest == digest {
			return true
		}
	}
	return false
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
	artifacts, found, err := DesiredArtifactsFromPod(pod)
	if err != nil || !found {
		return nodecache.DesiredArtifact{}, found, err
	}
	return artifacts[0], true, nil
}

func DesiredArtifactsFromPod(pod *corev1.Pod) ([]nodecache.DesiredArtifact, bool, error) {
	if pod == nil {
		return nil, false, errors.New("node cache runtime pod must not be nil")
	}
	annotations := pod.GetAnnotations()
	deliveryMode := strings.TrimSpace(annotations[modeldelivery.ResolvedDeliveryModeAnnotation])
	deliveryReason := strings.TrimSpace(annotations[modeldelivery.ResolvedDeliveryReasonAnnotation])
	if deliveryMode != string(modeldelivery.DeliveryModeSharedDirect) ||
		deliveryReason != string(modeldelivery.DeliveryReasonNodeSharedRuntimePlane) {
		return nil, false, nil
	}
	if models := strings.TrimSpace(annotations[modeldelivery.ResolvedModelsAnnotation]); models != "" {
		artifacts, err := desiredArtifactsFromResolvedModels(models)
		if err != nil {
			return nil, false, err
		}
		return artifacts, len(artifacts) > 0, nil
	}
	digest := strings.TrimSpace(annotations[modeldelivery.ResolvedDigestAnnotation])
	artifactURI := strings.TrimSpace(annotations[modeldelivery.ResolvedArtifactURIAnnotation])
	if digest == "" || artifactURI == "" {
		return nil, false, errors.New("managed pod shared-direct artifact annotations are incomplete")
	}
	artifacts, err := nodecache.NormalizeDesiredArtifacts([]nodecache.DesiredArtifact{{
		ArtifactURI: artifactURI,
		Digest:      digest,
		Family:      strings.TrimSpace(annotations[modeldelivery.ResolvedArtifactFamilyAnnotation]),
	}})
	if err != nil {
		return nil, false, err
	}
	return artifacts, true, nil
}

type resolvedModelAnnotation struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Family string `json:"family,omitempty"`
}

func desiredArtifactsFromResolvedModels(value string) ([]nodecache.DesiredArtifact, error) {
	var entries []resolvedModelAnnotation
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return nil, err
	}
	artifacts := make([]nodecache.DesiredArtifact, 0, len(entries))
	for _, entry := range entries {
		artifacts = append(artifacts, nodecache.DesiredArtifact{
			ArtifactURI: entry.URI,
			Digest:      entry.Digest,
			Family:      entry.Family,
		})
	}
	return nodecache.NormalizeDesiredArtifacts(artifacts)
}
