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

package directuploadstate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	client    kubernetes.Interface
	namespace string
	name      string
}

func New(client kubernetes.Interface, namespace, name string) (*Client, error) {
	switch {
	case client == nil:
		return nil, errors.New("direct upload state client must not be nil")
	case strings.TrimSpace(namespace) == "":
		return nil, errors.New("direct upload state namespace must not be empty")
	case strings.TrimSpace(name) == "":
		return nil, errors.New("direct upload state secret name must not be empty")
	}
	return &Client{
		client:    client,
		namespace: strings.TrimSpace(namespace),
		name:      strings.TrimSpace(name),
	}, nil
}

func NewInCluster(namespace, name string) (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return New(clientset, namespace, name)
}

func (c *Client) Load(ctx context.Context) (modelpackports.DirectUploadState, bool, error) {
	secret, found, err := c.get(ctx)
	if err != nil || !found {
		return modelpackports.DirectUploadState{}, found, err
	}
	state, err := StateFromSecret(secret)
	if err != nil {
		return modelpackports.DirectUploadState{}, false, err
	}
	return state, true, nil
}

func (c *Client) Save(ctx context.Context, state modelpackports.DirectUploadState) error {
	return c.update(ctx, func(secret *corev1.Secret) error {
		payload, err := marshalState(state)
		if err != nil {
			return err
		}
		if secret.Data == nil {
			secret.Data = make(map[string][]byte, 1)
		}
		secret.Data[stateKey] = payload
		return nil
	})
}

func (c *Client) get(ctx context.Context) (*corev1.Secret, bool, error) {
	if c == nil {
		return nil, false, errors.New("direct upload state client must not be nil")
	}

	secret, err := c.client.CoreV1().Secrets(c.namespace).Get(ctx, c.name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return secret, true, nil
}

func (c *Client) update(ctx context.Context, mutate func(secret *corev1.Secret) error) error {
	secret, found, err := c.get(ctx)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("direct upload state secret %s/%s not found", c.namespace, c.name)
	}
	if err := mutate(secret); err != nil {
		return err
	}
	_, err = c.client.CoreV1().Secrets(c.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		return c.update(ctx, mutate)
	}
	return err
}
