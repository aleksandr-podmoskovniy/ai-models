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

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	deliverycontract "github.com/deckhouse/ai-models/controller/internal/workloaddelivery"
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
	client          kubernetes.Interface
	deliveryAuthKey string
}

func NewDesiredArtifactsClient(client kubernetes.Interface) (*DesiredArtifactsClient, error) {
	if client == nil {
		return nil, errors.New("node cache runtime desired artifacts client must not be nil")
	}
	return NewDesiredArtifactsClientWithAuthKey(client, "")
}

func NewDesiredArtifactsClientWithAuthKey(client kubernetes.Interface, deliveryAuthKey string) (*DesiredArtifactsClient, error) {
	if client == nil {
		return nil, errors.New("node cache runtime desired artifacts client must not be nil")
	}
	return &DesiredArtifactsClient{
		client:          client,
		deliveryAuthKey: strings.TrimSpace(deliveryAuthKey),
	}, nil
}

func NewInClusterDesiredArtifactsClient(deliveryAuthKey string) (*DesiredArtifactsClient, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewDesiredArtifactsClientWithAuthKey(clientset, deliveryAuthKey)
}

func (c *DesiredArtifactsClient) LoadNodeDesiredArtifacts(ctx context.Context, nodeName string) ([]nodecache.DesiredArtifact, error) {
	if c == nil {
		return nil, errors.New("node cache runtime desired artifacts client must not be nil")
	}

	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return nil, errors.New("node cache runtime node name must not be empty")
	}

	pods, err := c.scheduledPods(ctx, nodeName)
	if err != nil {
		return nil, err
	}
	return desiredArtifactsFromScheduledPods(pods, nodeName, c.deliveryAuthKey)
}

func (c *DesiredArtifactsClient) scheduledPods(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	podList, err := c.client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(podNodeNameFieldSelector, nodeName).String(),
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func desiredArtifactsFromScheduledPods(pods []corev1.Pod, nodeName, deliveryAuthKey string) ([]nodecache.DesiredArtifact, error) {
	artifacts := make([]nodecache.DesiredArtifact, 0, len(pods))
	for index := range pods {
		pod := &pods[index]
		if strings.TrimSpace(pod.Spec.NodeName) != nodeName || !IsActiveScheduledPod(pod) {
			continue
		}
		podArtifacts, found, err := VerifiedDesiredArtifactsFromPod(pod, deliveryAuthKey)
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
	artifacts, found, err := VerifiedDesiredArtifactsFromPod(pod, c.deliveryAuthKey)
	if err != nil || !found {
		return false, err
	}
	return desiredArtifactsContainDigest(artifacts, digest), nil
}

func (c *DesiredArtifactsClient) csiPublishPod(ctx context.Context, nodeName string, attributes map[string]string, digest string) (*corev1.Pod, bool, error) {
	request, ok := csiPublishRequestFromAttributes(nodeName, attributes, digest)
	if !ok {
		return nil, false, nil
	}
	return c.loadActiveCSIPublishPod(ctx, request)
}

type csiPublishRequest struct {
	NodeName     string
	PodName      string
	PodNamespace string
	PodUID       string
	Digest       string
}

func csiPublishRequestFromAttributes(nodeName string, attributes map[string]string, digest string) (csiPublishRequest, bool) {
	request := csiPublishRequest{
		NodeName:     strings.TrimSpace(nodeName),
		PodName:      strings.TrimSpace(attributes[csiPodNameAttribute]),
		PodNamespace: strings.TrimSpace(attributes[csiPodNamespaceAttribute]),
		PodUID:       strings.TrimSpace(attributes[csiPodUIDAttribute]),
		Digest:       strings.TrimSpace(digest),
	}
	if request.NodeName == "" || request.PodName == "" || request.PodNamespace == "" || request.PodUID == "" || request.Digest == "" {
		return csiPublishRequest{}, false
	}
	return request, true
}

func (c *DesiredArtifactsClient) loadActiveCSIPublishPod(ctx context.Context, request csiPublishRequest) (*corev1.Pod, bool, error) {
	pod, err := c.client.CoreV1().Pods(request.PodNamespace).Get(ctx, request.PodName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if string(pod.UID) != request.PodUID || strings.TrimSpace(pod.Spec.NodeName) != request.NodeName || !IsActiveScheduledPod(pod) {
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
	resolved, found, err := deliverycontract.ResolvedArtifactsFromAnnotations(pod.GetAnnotations())
	if err != nil {
		return nil, false, err
	}
	if found {
		artifacts, err := desiredArtifactsFromResolvedArtifacts(resolved)
		if err != nil {
			return nil, false, err
		}
		return artifacts, true, nil
	}
	if deliverycontract.IsSharedDirectResolvedDelivery(pod.GetAnnotations()) {
		return nil, false, errors.New("managed pod shared-direct artifact annotations are incomplete")
	}
	return nil, false, nil
}

func VerifiedDesiredArtifactsFromPod(pod *corev1.Pod, deliveryAuthKey string) ([]nodecache.DesiredArtifact, bool, error) {
	deliveryAuthKey = strings.TrimSpace(deliveryAuthKey)
	if deliveryAuthKey == "" {
		return DesiredArtifactsFromPod(pod)
	}
	if pod == nil {
		return nil, false, errors.New("node cache runtime pod must not be nil")
	}
	annotations := pod.GetAnnotations()
	if !deliverycontract.IsSharedDirectResolvedDelivery(annotations) {
		return nil, false, nil
	}
	if !deliverycontract.VerifyResolvedDeliverySignature(pod.GetNamespace(), annotations, deliveryAuthKey) {
		return nil, false, nil
	}
	return DesiredArtifactsFromPod(pod)
}

func desiredArtifactsFromResolvedArtifacts(resolved []deliverycontract.ResolvedArtifact) ([]nodecache.DesiredArtifact, error) {
	artifacts := make([]nodecache.DesiredArtifact, 0, len(resolved))
	for _, artifact := range resolved {
		artifacts = append(artifacts, nodecache.DesiredArtifact{
			ArtifactURI: artifact.ArtifactURI,
			Digest:      artifact.Digest,
			Family:      artifact.Family,
			SizeBytes:   artifact.SizeBytes,
		})
	}
	return nodecache.NormalizeDesiredArtifacts(artifacts)
}
