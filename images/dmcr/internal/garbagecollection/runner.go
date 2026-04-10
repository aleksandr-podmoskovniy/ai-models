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

package garbagecollection

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	RequestLabelKey       = "ai-models.deckhouse.io/dmcr-gc-request"
	RequestLabelValue     = "true"
	switchAnnotationKey   = "ai-models.deckhouse.io/dmcr-gc-switch"
	doneAnnotationKey     = "ai-models.deckhouse.io/dmcr-gc-done"
	DefaultRegistryBinary = "/usr/bin/dmcr"
	DefaultConfigPath     = "/etc/docker/registry/config.yml"
	DefaultRescanInterval = 5 * time.Second
)

type Options struct {
	RequestNamespace     string
	RequestLabelSelector string
	RegistryBinary       string
	ConfigPath           string
	GCTimeout            time.Duration
	RescanInterval       time.Duration
}

func DefaultRequestLabelSelector() string {
	return RequestLabelKey + "=" + RequestLabelValue
}

func NewInClusterClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("build in-cluster config: %w", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes client: %w", err)
	}
	return client, nil
}

func RunLoop(ctx context.Context, client kubernetes.Interface, options Options) error {
	if client == nil {
		return fmt.Errorf("kubernetes client must not be nil")
	}

	options = applyDefaultOptions(options)

	signalContext, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	for {
		pendingSecrets, err := listPendingRequestSecrets(signalContext, client, options.RequestNamespace, options.RequestLabelSelector)
		if err != nil {
			return err
		}
		if len(pendingSecrets) == 0 {
			select {
			case <-signalContext.Done():
				return nil
			case <-time.After(options.RescanInterval):
				continue
			}
		}

		if _, err := execGarbageCollect(signalContext, options); err != nil {
			return err
		}
		if err := markRequestsDone(signalContext, client, options.RequestNamespace, pendingSecrets, time.Now().UTC()); err != nil {
			return err
		}
	}
}

func applyDefaultOptions(options Options) Options {
	if strings.TrimSpace(options.RequestLabelSelector) == "" {
		options.RequestLabelSelector = DefaultRequestLabelSelector()
	}
	if strings.TrimSpace(options.RegistryBinary) == "" {
		options.RegistryBinary = DefaultRegistryBinary
	}
	if strings.TrimSpace(options.ConfigPath) == "" {
		options.ConfigPath = DefaultConfigPath
	}
	if options.GCTimeout <= 0 {
		options.GCTimeout = 10 * time.Minute
	}
	if options.RescanInterval <= 0 {
		options.RescanInterval = DefaultRescanInterval
	}
	return options
}

func listPendingRequestSecrets(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) ([]corev1.Secret, error) {
	secretList, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list dmcr garbage-collection request secrets: %w", err)
	}

	pending := make([]corev1.Secret, 0, len(secretList.Items))
	for _, secret := range secretList.Items {
		if shouldRunGarbageCollection(secret) {
			pending = append(pending, secret)
		}
	}
	return pending, nil
}

func shouldRunGarbageCollection(secret corev1.Secret) bool {
	if secret.Labels[RequestLabelKey] != RequestLabelValue {
		return false
	}
	return strings.TrimSpace(secret.Annotations[switchAnnotationKey]) != ""
}

func execGarbageCollect(ctx context.Context, options Options) ([]byte, error) {
	gcContext, cancel := context.WithTimeout(ctx, options.GCTimeout)
	defer cancel()

	command := exec.CommandContext(gcContext, options.RegistryBinary, "garbage-collect", options.ConfigPath, "--delete-untagged")
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("run dmcr garbage-collect: %s", message)
	}
	return output, nil
}

func markRequestsDone(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	secrets []corev1.Secret,
	finishedAt time.Time,
) error {
	for _, secret := range secrets {
		secretCopy := secret.DeepCopy()
		if secretCopy.Annotations == nil {
			secretCopy.Annotations = make(map[string]string, 2)
		}
		delete(secretCopy.Annotations, switchAnnotationKey)
		secretCopy.Annotations[doneAnnotationKey] = finishedAt.Format(time.RFC3339Nano)

		if _, err := client.CoreV1().Secrets(namespace).Update(ctx, secretCopy, metav1.UpdateOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("mark dmcr garbage-collection request %s as done: %w", secretCopy.Name, err)
		}
	}
	return nil
}
