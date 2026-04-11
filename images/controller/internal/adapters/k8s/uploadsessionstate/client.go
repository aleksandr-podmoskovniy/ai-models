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

package uploadsessionstate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
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

func New(client kubernetes.Interface, namespace string) (*Client, error) {
	if client == nil {
		return nil, errors.New("upload session state client must not be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, errors.New("upload session state namespace must not be empty")
	}
	return &Client{
		client:    client,
		namespace: strings.TrimSpace(namespace),
	}, nil
}

func NewInCluster(namespace string) (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return New(clientset, namespace)
}

func (c *Client) Load(ctx context.Context, name string) (Session, bool, error) {
	secret, found, err := c.get(ctx, name)
	if err != nil || !found {
		return Session{}, found, err
	}
	session, err := SessionFromSecret(secret)
	if err != nil {
		return Session{}, false, err
	}
	return *session, true, nil
}

func (c *Client) SaveMultipart(ctx context.Context, name string, state uploadsessionruntime.SessionState) error {
	return c.update(ctx, name, func(secret *corev1.Secret) error {
		if err := setMultipartState(secret, state); err != nil {
			return err
		}
		secret.Data[phaseKey] = []byte(string(PhaseUploading))
		delete(secret.Data, failureMessageKey)
		delete(secret.Data, stagedHandleKey)
		return nil
	})
}

func (c *Client) SaveMultipartParts(ctx context.Context, name string, parts []uploadsessionruntime.UploadedPart) error {
	return c.update(ctx, name, func(secret *corev1.Secret) error {
		return setUploadedParts(secret, parts)
	})
}

func (c *Client) SaveProbe(ctx context.Context, name string, state uploadsessionruntime.ProbeState) error {
	fileName := strings.TrimSpace(state.FileName)
	if fileName == "" {
		return errors.New("upload session probe file name must not be empty")
	}
	return c.update(ctx, name, func(secret *corev1.Secret) error {
		ensureData(secret)
		secret.Data[stateProbeFileNameKey] = []byte(fileName)
		if resolved := strings.TrimSpace(string(state.ResolvedInputFormat)); resolved != "" {
			secret.Data[stateProbeFormatKey] = []byte(resolved)
		} else {
			delete(secret.Data, stateProbeFormatKey)
		}
		secret.Data[phaseKey] = []byte(string(PhaseProbing))
		delete(secret.Data, failureMessageKey)
		return nil
	})
}

func (c *Client) ClearMultipart(ctx context.Context, name string) error {
	return c.update(ctx, name, func(secret *corev1.Secret) error {
		ensureData(secret)
		clearMultipartState(secret)
		if strings.TrimSpace(string(secret.Data[stateProbeFileNameKey])) != "" {
			secret.Data[phaseKey] = []byte(string(PhaseProbing))
		} else {
			secret.Data[phaseKey] = []byte(string(PhaseIssued))
		}
		delete(secret.Data, failureMessageKey)
		delete(secret.Data, stagedHandleKey)
		return nil
	})
}

func (c *Client) MarkUploaded(ctx context.Context, name string, handle cleanuphandle.Handle) error {
	return c.update(ctx, name, func(secret *corev1.Secret) error {
		return MarkUploadedSecret(secret, handle)
	})
}

func (c *Client) MarkPublishing(ctx context.Context, name string) error {
	return c.update(ctx, name, MarkPublishingSecret)
}

func (c *Client) MarkCompleted(ctx context.Context, name string) error {
	return c.update(ctx, name, MarkCompletedSecret)
}

func (c *Client) MarkFailed(ctx context.Context, name string, message string) error {
	return c.markTerminal(ctx, name, PhaseFailed, message)
}

func (c *Client) MarkAborted(ctx context.Context, name string, message string) error {
	return c.markTerminal(ctx, name, PhaseAborted, message)
}

func (c *Client) MarkExpired(ctx context.Context, name string, message string) error {
	return c.markTerminal(ctx, name, PhaseExpired, message)
}

func (c *Client) markTerminal(ctx context.Context, name string, phase Phase, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("upload session terminal message must not be empty")
	}
	return c.update(ctx, name, func(secret *corev1.Secret) error {
		ensureData(secret)
		delete(secret.Data, stagedHandleKey)
		secret.Data[phaseKey] = []byte(string(phase))
		secret.Data[failureMessageKey] = []byte(message)
		return nil
	})
}

func (c *Client) get(ctx context.Context, name string) (*corev1.Secret, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false, errors.New("upload session state secret name must not be empty")
	}
	secret, err := c.client.CoreV1().Secrets(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return secret, true, nil
}

func (c *Client) update(ctx context.Context, name string, mutate func(secret *corev1.Secret) error) error {
	secret, found, err := c.get(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("upload session secret %s/%s not found", c.namespace, name)
	}
	if err := mutate(secret); err != nil {
		return err
	}
	_, err = c.client.CoreV1().Secrets(c.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		return c.update(ctx, name, mutate)
	}
	return err
}
