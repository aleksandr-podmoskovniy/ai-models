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

package uploadsession

import (
	"context"
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) MarkPublishing(ctx context.Context, ownerUID types.UID) error {
	return s.mutateSessionSecretByOwnerUID(ctx, ownerUID, uploadsessionstate.MarkPublishingSecret)
}

func (s *Service) MarkCompleted(ctx context.Context, ownerUID types.UID) error {
	return s.mutateSessionSecretByOwnerUID(ctx, ownerUID, uploadsessionstate.MarkCompletedSecret)
}

func (s *Service) MarkFailed(ctx context.Context, ownerUID types.UID, message string) error {
	return s.mutateSessionSecretByOwnerUID(ctx, ownerUID, func(secret *corev1.Secret) error {
		return uploadsessionstate.MarkPublishingFailedSecret(secret, message)
	})
}

func (s *Service) mutateSessionSecretByOwnerUID(
	ctx context.Context,
	ownerUID types.UID,
	mutate func(secret *corev1.Secret) error,
) error {
	if s == nil {
		return errors.New("upload session service must not be nil")
	}
	if strings.TrimSpace(string(ownerUID)) == "" {
		return errors.New("upload session owner UID must not be empty")
	}
	if mutate == nil {
		return errors.New("upload session secret mutation must not be nil")
	}

	name, err := resourcenames.UploadSessionSecretName(ownerUID)
	if err != nil {
		return err
	}

	var secret corev1.Secret
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: s.options.Runtime.Namespace}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := mutate(&secret); err != nil {
		return err
	}
	return s.client.Update(ctx, &secret)
}
