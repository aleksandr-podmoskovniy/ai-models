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

package sourceworker

import (
	"context"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) deleteResources(ctx context.Context, pod *corev1.Pod, directUploadState modelpackports.DirectUploadState) error {
	if s == nil || pod == nil {
		return nil
	}
	ownerUID, ok := resourcenames.OwnerUIDFromLabels(pod.Labels)
	if ok {
		if err := ociregistry.DeleteProjectedAccess(ctx, s.client, s.options.Namespace, ownerUID); err != nil {
			return err
		}
	}
	secret, err := s.projectedAuthSecretForPod(pod)
	if err != nil {
		return err
	}
	resources := []client.Object{secret, pod}
	if shouldDeleteDirectUploadState(pod, directUploadState) {
		stateSecret, err := sourceWorkerStateSecret(s.options.Namespace, ownerUID)
		if err != nil {
			return err
		}
		if stateSecret != nil {
			resources = append(resources, stateSecret)
		}
	}
	return ownedresource.DeleteAll(ctx, s.client, resources...)
}

func shouldDeleteDirectUploadState(pod *corev1.Pod, directUploadState modelpackports.DirectUploadState) bool {
	if pod == nil {
		return false
	}
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return true
	case corev1.PodFailed:
		return directUploadState.Phase != modelpackports.DirectUploadStatePhaseRunning
	default:
		return false
	}
}

func sourceWorkerStateSecret(namespace string, ownerUID types.UID) (*corev1.Secret, error) {
	if strings.TrimSpace(string(ownerUID)) == "" {
		return nil, nil
	}
	name, err := resourcenames.SourceWorkerStateSecretName(ownerUID)
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: strings.TrimSpace(namespace),
		},
	}, nil
}
