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
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (s *Service) ensureProjectedAuthSecret(
	ctx context.Context,
	ownerObject client.Object,
	owner publicationports.Owner,
	plan publicationapp.SourceWorkerPlan,
) (string, error) {
	authRef := sourceAuthSecretRef(plan)
	if authRef == nil {
		return "", nil
	}

	projectedData, err := s.projectedAuthSecretData(ctx, plan, *authRef)
	if err != nil {
		return "", err
	}

	secretName, err := resourcenames.SourceWorkerAuthSecretName(owner.UID)
	if err != nil {
		return "", err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: s.options.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, s.client, secret, func() error {
		secret.Labels = buildLabels(owner)
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = projectedData
		return ownedresource.MaybeSetControllerReference(ownerObject, secret, s.scheme)
	}); err != nil {
		return "", err
	}

	return secret.Name, nil
}

func (s *Service) projectedAuthSecretData(
	ctx context.Context,
	plan publicationapp.SourceWorkerPlan,
	ref publicationapp.SourceAuthSecretRef,
) (map[string][]byte, error) {
	sourceSecret := &corev1.Secret{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, sourceSecret); err != nil {
		return nil, err
	}

	switch {
	case plan.HuggingFace != nil && plan.HuggingFace.AuthSecretRef != nil:
		token, err := projectedHFToken(sourceSecret)
		if err != nil {
			return nil, err
		}
		return map[string][]byte{
			"token": token,
		}, nil
	default:
		return nil, errors.New("source worker auth projection requires a HuggingFace auth plan")
	}
}

func sourceAuthSecretRef(plan publicationapp.SourceWorkerPlan) *publicationapp.SourceAuthSecretRef {
	switch {
	case plan.HuggingFace != nil:
		return plan.HuggingFace.AuthSecretRef
	default:
		return nil
	}
}

func projectedHFToken(secret *corev1.Secret) ([]byte, error) {
	for _, key := range []string{"token", "HF_TOKEN", "HUGGING_FACE_HUB_TOKEN"} {
		if value := trimSecretValue(secret.Data[key]); len(value) > 0 {
			return value, nil
		}
	}
	return nil, fmt.Errorf(
		"source worker huggingFace auth secret %s/%s must contain one of: token, HF_TOKEN, HUGGING_FACE_HUB_TOKEN",
		secret.Namespace,
		secret.Name,
	)
}

func trimSecretValue(value []byte) []byte {
	return bytes.TrimSpace(value)
}
