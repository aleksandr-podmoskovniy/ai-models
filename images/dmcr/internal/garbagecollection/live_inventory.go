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
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	cleanupStateAppName  = "ai-models-cleanup-state"
	cleanupHandleDataKey = "cleanupHandle"
	appNameLabelKey      = "app.kubernetes.io/name"
)

type cleanupHandleSnapshot struct {
	Kind    string                        `json:"kind,omitempty"`
	Backend *cleanupHandleBackendSnapshot `json:"backend,omitempty"`
}

type cleanupHandleBackendSnapshot struct {
	Reference                string `json:"reference,omitempty"`
	RepositoryMetadataPrefix string `json:"repositoryMetadataPrefix,omitempty"`
	SourceMirrorPrefix       string `json:"sourceMirrorPrefix,omitempty"`
}

func DiscoverLivePrefixes(ctx context.Context, client kubernetes.Interface, namespace string) (livePrefixSet, error) {
	if client == nil {
		return livePrefixSet{}, fmt.Errorf("kubernetes client must not be nil")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return livePrefixSet{}, fmt.Errorf("cleanup state namespace must not be empty")
	}

	live := newLivePrefixSet()
	secrets, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: appNameLabelKey + "=" + cleanupStateAppName,
	})
	if err != nil {
		return livePrefixSet{}, err
	}
	for _, secret := range secrets.Items {
		if err := collectLivePrefixesFromSecret(&secret, &live); err != nil {
			return livePrefixSet{}, err
		}
	}
	return live, nil
}

func collectLivePrefixesFromSecret(secret *corev1.Secret, live *livePrefixSet) error {
	if live == nil {
		return fmt.Errorf("live prefix set must not be nil")
	}
	if secret == nil {
		return nil
	}
	rawHandle := strings.TrimSpace(string(secret.Data[cleanupHandleDataKey]))
	if rawHandle == "" {
		return nil
	}

	var handle cleanupHandleSnapshot
	if err := json.Unmarshal([]byte(rawHandle), &handle); err != nil {
		return fmt.Errorf("decode cleanup state %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	if strings.TrimSpace(handle.Kind) != "BackendArtifact" || handle.Backend == nil {
		return nil
	}

	if prefix := strings.Trim(strings.TrimSpace(handle.Backend.RepositoryMetadataPrefix), "/"); prefix != "" {
		live.addRepository(prefix)
	} else if prefix := repositoryMetadataPrefixFromReference(handle.Backend.Reference); prefix != "" {
		live.addRepository(prefix)
	}
	if prefix := strings.Trim(strings.TrimSpace(handle.Backend.SourceMirrorPrefix), "/"); prefix != "" {
		live.addRaw(prefix)
	}
	return nil
}
