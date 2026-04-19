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
	"context"
	"errors"
	"strings"

	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	client    kubernetes.Interface
	namespace string
}

func NewClient(client kubernetes.Interface, namespace string) (*Client, error) {
	if client == nil {
		return nil, errors.New("node cache intent client must not be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, errors.New("node cache intent namespace must not be empty")
	}
	return &Client{client: client, namespace: strings.TrimSpace(namespace)}, nil
}

func NewInClusterClient(namespace string) (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewClient(clientset, namespace)
}

func (c *Client) LoadNodeIntents(ctx context.Context, nodeName string) ([]intentcontract.ArtifactIntent, error) {
	if c == nil {
		return nil, errors.New("node cache intent client must not be nil")
	}
	name, err := configMapName(nodeName)
	if err != nil {
		return nil, err
	}
	configMap, err := c.client.CoreV1().ConfigMaps(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeConfigMapIntents(configMap)
}

func configMapName(nodeName string) (string, error) {
	return resourcenames.NodeCacheIntentConfigMapName(nodeName)
}

func decodeConfigMapIntents(configMap *corev1.ConfigMap) ([]intentcontract.ArtifactIntent, error) {
	if configMap == nil {
		return nil, errors.New("node cache intent configmap must not be nil")
	}
	return intentcontract.DecodeIntents([]byte(configMap.Data[intentcontract.DataKey]))
}
